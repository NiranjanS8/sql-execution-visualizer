// Package metrics records what actually happened during one execution of a plan.
//
// A Recorder is created per query run and owns all of its state. Separate runs
// share no mutable state, so concurrent queries are safe; a single Recorder is
// owned by the goroutine that drains its operator and must not be used by
// several goroutines at once.
//
// Units and semantics:
//
//   - Durations are wall-clock time.Duration values. A node's Duration is its
//     exclusive time: the time spent inside its own Next calls, excluding time
//     spent waiting on its child. The trace Duration runs from the first
//     operator call to Finish.
//   - RowsIn counts rows the node successfully pulled from its child. RowsOut
//     counts rows the node successfully returned to its parent. Scan has no
//     child, so its RowsIn is zero.
//   - Samples retain copies of at most SampleLimit rows per point. Truncated is
//     true when at least one row was not retained, including when sampling is
//     disabled but rows flowed.
//   - Statuses describe what each operator observed. An operator that was never
//     pulled is not_started; one that returned io.EOF is completed; one that
//     stopped because an ancestor stopped pulling is stopped; one that observed
//     an error is failed; one that observed context cancellation is cancelled.
package metrics

import (
	"time"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/planner"
)

// Status is the lifecycle state of one operator in a run.
type Status string

const (
	StatusNotStarted Status = "not_started"
	StatusRunning    Status = "running"
	StatusCompleted  Status = "completed"
	StatusStopped    Status = "stopped"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

// Options controls sampling. The zero value records metrics without retaining
// any rows.
type Options struct {
	// SampleLimit is the maximum number of rows retained at each sample point.
	// A negative value is rejected; zero disables sampling.
	SampleLimit int
	// CaptureInput retains rows entering an operator.
	CaptureInput bool
	// CaptureOutput retains rows leaving an operator.
	CaptureOutput bool
}

// DefaultOptions samples up to five rows at both the input and output of every
// operator.
func DefaultOptions() Options {
	return Options{
		SampleLimit:   5,
		CaptureInput:  true,
		CaptureOutput: true,
	}
}

// Sample is a bounded, copied set of rows. Truncated reports that at least one
// row at this point was not retained.
type Sample struct {
	Rows      []catalog.Row
	Truncated bool
}

// Edge is one parent-to-child plan edge.
type Edge struct {
	From string
	To   string
}

// Node is one operator's recorded state. Nodes are ordered by their stable ID.
type Node struct {
	ID       string
	Kind     planner.Kind
	Label    string
	ParentID string
	Children []string
	Schema   []catalog.Column

	RowsIn   int64
	RowsOut  int64
	Duration time.Duration

	Status Status
	Error  string

	Input  Sample
	Output Sample
}

// Trace is a complete graph of one run: every operator, every edge, the root,
// and the total duration. It contains copies, so later execution state cannot
// change it.
type Trace struct {
	RootID   string
	Nodes    []Node
	Edges    []Edge
	Duration time.Duration
	Status   Status
	Error    string
}
