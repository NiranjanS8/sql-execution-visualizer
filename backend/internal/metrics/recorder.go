package metrics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/execution"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/planner"
)

// Recorder instruments one execution of a plan. IDs are assigned in pre-order
// from the root, so the same plan shape always produces the same IDs. The
// recorder is not safe for concurrent use by multiple goroutines; each query run
// owns its own recorder.
type Recorder struct {
	opts     Options
	root     *nodeState
	order    []*nodeState
	operator execution.Operator

	started  time.Time
	finished bool
	total    time.Duration
	trace    Trace
	stack    []*frame
}

type nodeState struct {
	id       string
	planNode planner.Node
	parent   *nodeState
	children []*nodeState

	rowsIn   int64
	rowsOut  int64
	duration time.Duration

	started bool
	status  Status
	err     error

	input  []catalog.Row
	output []catalog.Row
}

type frame struct {
	start time.Time
	child time.Duration
}

// NewRecorder builds the instrumented operator chain for plan. The plan is
// validated before any operator is built, so a broken plan fails here rather
// than midway through execution.
func NewRecorder(plan *planner.Plan, opts Options) (*Recorder, error) {
	if plan == nil || plan.Root == nil {
		return nil, fmt.Errorf("metrics: plan has no root")
	}
	if opts.SampleLimit < 0 {
		return nil, fmt.Errorf("metrics: sample limit must not be negative, got %d", opts.SampleLimit)
	}
	if err := plan.Validate(); err != nil {
		return nil, err
	}

	recorder := &Recorder{opts: opts}
	recorder.root = recorder.assign(plan.Root, nil)

	operator, err := recorder.build(recorder.root)
	if err != nil {
		return nil, err
	}
	recorder.operator = operator
	return recorder, nil
}

// Operator returns the instrumented root. Draining it records the run.
func (r *Recorder) Operator() execution.Operator { return r.operator }

// Finish finalizes the run and returns a snapshot of the trace. It must be
// called once after draining Operator; later calls return the same snapshot.
// err is the error that ended the drain, or nil when the operator was exhausted.
func (r *Recorder) Finish(err error) Trace {
	if r.finished {
		return r.trace
	}
	r.finished = true
	if !r.started.IsZero() {
		r.total = time.Since(r.started)
	}

	cancelled := isCancellation(err)
	if err != nil && !r.root.terminal() {
		r.root.status = failureStatus(err)
		r.root.err = err
	}

	for _, state := range r.order {
		if !state.started {
			continue
		}
		if state.status != StatusRunning {
			continue
		}
		if cancelled {
			state.status = StatusCancelled
			state.err = err
			continue
		}
		state.status = StatusStopped
	}

	r.trace = r.snapshot(err)
	return r.trace
}

// Execute runs plan and returns its rows and trace. Instrumentation wraps the
// same operators that an uninstrumented run would use, so the rows are identical.
func Execute(ctx context.Context, plan *planner.Plan, opts Options) ([]catalog.Row, Trace, error) {
	recorder, err := NewRecorder(plan, opts)
	if err != nil {
		return nil, Trace{}, err
	}

	rows, runErr := execution.Collect(ctx, recorder.Operator())
	return rows, recorder.Finish(runErr), runErr
}

func (r *Recorder) assign(node planner.Node, parent *nodeState) *nodeState {
	state := &nodeState{
		id:       fmt.Sprintf("n%d", len(r.order)+1),
		planNode: node,
		parent:   parent,
		status:   StatusNotStarted,
	}
	r.order = append(r.order, state)

	for _, child := range node.Children() {
		state.children = append(state.children, r.assign(child, state))
	}
	return state
}

func (r *Recorder) build(state *nodeState) (execution.Operator, error) {
	children := state.children
	if len(children) > 1 {
		return nil, fmt.Errorf("metrics: node %s has %d inputs; only one is supported", state.id, len(children))
	}

	var input execution.Operator
	if len(children) == 1 {
		child, err := r.build(children[0])
		if err != nil {
			return nil, err
		}
		input = &inputTap{run: r, state: state, input: child}
	}

	physical, err := planner.BuildOperator(state.planNode, input)
	if err != nil {
		return nil, err
	}
	return &instrumented{run: r, state: state, op: physical}, nil
}

func (r *Recorder) begin() {
	if r.started.IsZero() {
		r.started = time.Now()
	}
}

func (r *Recorder) enter() *frame {
	active := &frame{start: time.Now()}
	r.stack = append(r.stack, active)
	return active
}

func (r *Recorder) leave(active *frame) (elapsed, child time.Duration) {
	elapsed = time.Since(active.start)
	r.stack = r.stack[:len(r.stack)-1]
	if len(r.stack) > 0 {
		r.stack[len(r.stack)-1].child += elapsed
	}
	return elapsed, active.child
}

func (r *Recorder) snapshot(err error) Trace {
	trace := Trace{
		RootID:   r.root.id,
		Duration: r.total,
		Status:   r.root.status,
		Nodes:    make([]Node, len(r.order)),
	}

	for i, state := range r.order {
		trace.Nodes[i] = state.snapshot()
		for _, child := range state.children {
			trace.Edges = append(trace.Edges, Edge{From: state.id, To: child.id})
		}
	}

	if err != nil {
		trace.Error = err.Error()
	} else if r.root.err != nil {
		trace.Error = r.root.err.Error()
	}
	return trace
}

func (s *nodeState) start() {
	if s.started {
		return
	}
	s.started = true
	s.status = StatusRunning
}

func (s *nodeState) terminal() bool {
	switch s.status {
	case StatusCompleted, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

func (s *nodeState) captureInput(row catalog.Row, opts Options) {
	if opts.SampleLimit <= 0 || !opts.CaptureInput {
		return
	}
	if len(s.input) >= opts.SampleLimit {
		return
	}
	s.input = append(s.input, copyRow(row))
}

func (s *nodeState) captureOutput(row catalog.Row, opts Options) {
	if opts.SampleLimit <= 0 || !opts.CaptureOutput {
		return
	}
	if len(s.output) >= opts.SampleLimit {
		return
	}
	s.output = append(s.output, copyRow(row))
}

func (s *nodeState) snapshot() Node {
	node := Node{
		ID:       s.id,
		Kind:     s.planNode.Kind(),
		Label:    s.planNode.Describe(),
		Schema:   s.planNode.Schema().Columns(),
		RowsIn:   s.rowsIn,
		RowsOut:  s.rowsOut,
		Duration: s.duration,
		Status:   s.status,
		Children: make([]string, len(s.children)),
		Input: Sample{
			Rows:      copyRows(s.input),
			Truncated: s.rowsIn > int64(len(s.input)),
		},
		Output: Sample{
			Rows:      copyRows(s.output),
			Truncated: s.rowsOut > int64(len(s.output)),
		},
	}
	if s.parent != nil {
		node.ParentID = s.parent.id
	}
	for i, child := range s.children {
		node.Children[i] = child.id
	}
	if s.err != nil {
		node.Error = s.err.Error()
	}
	return node
}

func copyRow(row catalog.Row) catalog.Row {
	return append(catalog.Row(nil), row...)
}

func copyRows(rows []catalog.Row) []catalog.Row {
	if len(rows) == 0 {
		return nil
	}
	copied := make([]catalog.Row, len(rows))
	for i, row := range rows {
		copied[i] = copyRow(row)
	}
	return copied
}

func isCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func failureStatus(err error) Status {
	if isCancellation(err) {
		return StatusCancelled
	}
	return StatusFailed
}

type inputTap struct {
	run   *Recorder
	state *nodeState
	input execution.Operator
}

func (t *inputTap) Schema() catalog.Schema { return t.input.Schema() }

func (t *inputTap) Next(ctx context.Context) (catalog.Row, error) {
	row, err := t.input.Next(ctx)
	if err == nil {
		t.state.rowsIn++
		t.state.captureInput(row, t.run.opts)
	}
	return row, err
}

type instrumented struct {
	run   *Recorder
	state *nodeState
	op    execution.Operator
}

func (i *instrumented) Schema() catalog.Schema { return i.op.Schema() }

func (i *instrumented) Next(ctx context.Context) (catalog.Row, error) {
	i.run.begin()
	i.state.start()

	active := i.run.enter()
	row, err := i.op.Next(ctx)
	elapsed, child := i.run.leave(active)

	self := elapsed - child
	if self < 0 {
		self = 0
	}
	i.state.duration += self

	switch {
	case err == nil:
		i.state.rowsOut++
		i.state.captureOutput(row, i.run.opts)
	case errors.Is(err, io.EOF):
		i.state.status = StatusCompleted
	case isCancellation(err):
		i.state.status = StatusCancelled
		i.state.err = err
	default:
		i.state.status = StatusFailed
		i.state.err = err
	}

	return row, err
}
