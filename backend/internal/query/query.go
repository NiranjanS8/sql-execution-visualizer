// Package query orchestrates one synchronous SELECT: parse, resolve, plan,
// execute under limits, and report the result together with its execution trace.
//
// It is the only package that composes the SQL pipeline. The API layer owns
// transport concerns; the query package owns limits, error classification, and
// the decision to stop a run that exceeds its result budget.
package query

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/execution"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/metrics"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/planner"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/validate"
)

// ErrResultLimit is returned when a run produces more rows than the configured
// result budget. It is a sentinel so the trace and the API can classify the
// failure without parsing an error string.
var ErrResultLimit = errors.New("query result exceeds the configured row limit")

// Limits bound one query run. The zero value is not meaningful; use
// DefaultLimits or NewRunner, which fills in defaults for invalid values.
type Limits struct {
	MaxSQLLength int
	Timeout      time.Duration
	MaxRows      int
	Samples      metrics.Options
}

// DefaultLimits are the local-development budgets. They are intentionally
// conservative: the API is local, but a runaway sort or a wildcard over a large
// import should still fail loudly instead of freezing the browser.
func DefaultLimits() Limits {
	return Limits{
		MaxSQLLength: 10_000,
		Timeout:      5 * time.Second,
		MaxRows:      1_000,
		Samples:      metrics.DefaultOptions(),
	}
}

// Result is one completed run. Rows are the projected result rows; Trace records
// what every operator did while producing them.
type Result struct {
	RunID    string
	Status   metrics.Status
	Columns  []catalog.Column
	Rows     []catalog.Row
	Trace    metrics.Trace
	Duration time.Duration
}

// Runner executes queries against one catalog at a time. It is safe for
// concurrent use: the only shared state is a monotonic run counter.
type Runner struct {
	limits Limits
	seq    atomic.Uint64
}

// NewRunner builds a runner, replacing non-positive budgets with the defaults.
// A negative sample limit is clamped to zero, which disables sampling.
func NewRunner(limits Limits) *Runner {
	defaults := DefaultLimits()
	if limits.MaxSQLLength <= 0 {
		limits.MaxSQLLength = defaults.MaxSQLLength
	}
	if limits.Timeout <= 0 {
		limits.Timeout = defaults.Timeout
	}
	if limits.MaxRows <= 0 {
		limits.MaxRows = defaults.MaxRows
	}
	if limits.Samples.SampleLimit < 0 {
		limits.Samples.SampleLimit = 0
	}
	return &Runner{limits: limits}
}

// Limits returns the effective budgets after defaults were applied.
func (r *Runner) Limits() Limits { return r.limits }

// Execute runs one SELECT against tables. A parse, resolution, or type failure
// is returned before any operator runs. An execution failure, timeout, or result
// limit is returned after the trace is finished, so the caller can log the
// cause without exposing it to the browser.
func (r *Runner) Execute(ctx context.Context, sql string, tables *catalog.Catalog) (*Result, error) {
	started := time.Now()

	if tables == nil {
		return nil, fmt.Errorf("query: tables is nil")
	}
	if length := utf8.RuneCountInString(sql); length > r.limits.MaxSQLLength {
		return nil, &Error{
			Code:    CodeInvalidRequest,
			Message: fmt.Sprintf("SQL is %d characters; the limit is %d.", length, r.limits.MaxSQLLength),
		}
	}

	stmt, err := sqlparse.Parse(sql)
	if err != nil {
		return nil, classifyPipelineError(err)
	}

	validated, err := validate.Resolve(stmt, tables)
	if err != nil {
		return nil, classifyPipelineError(err)
	}

	plan, err := planner.Build(validated)
	if err != nil {
		return nil, fmt.Errorf("query: build plan: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, r.limits.Timeout)
	defer cancel()

	recorder, err := metrics.NewRecorder(plan, r.limits.Samples)
	if err != nil {
		return nil, fmt.Errorf("query: build recorder: %w", err)
	}

	rows, runErr := drain(runCtx, recorder.Operator(), r.limits.MaxRows)
	trace := recorder.Finish(runErr)

	if runErr != nil {
		return nil, r.classifyRunError(runErr)
	}

	return &Result{
		RunID:    r.nextRunID(),
		Status:   trace.Status,
		Columns:  plan.Root.Schema().Columns(),
		Rows:     rows,
		Trace:    trace,
		Duration: time.Since(started),
	}, nil
}

// drain pulls rows until EOF, an error, or one row past the result budget. The
// extra pull is what distinguishes "exactly at the limit" from "over the limit";
// the extra row is discarded and never returned as a partial result.
func drain(ctx context.Context, op execution.Operator, maxRows int) ([]catalog.Row, error) {
	rows := make([]catalog.Row, 0, min(maxRows, 64))

	for {
		row, err := op.Next(ctx)
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		if len(rows) >= maxRows {
			return nil, ErrResultLimit
		}
		rows = append(rows, row)
	}
}

func (r *Runner) classifyRunError(err error) error {
	switch {
	case errors.Is(err, ErrResultLimit):
		return &Error{
			Code: CodeResultLimit,
			Message: fmt.Sprintf(
				"Query returned more than %d rows. Add a LIMIT clause or narrow the query.",
				r.limits.MaxRows,
			),
		}
	case errors.Is(err, context.DeadlineExceeded):
		return &Error{
			Code:    CodeTimeout,
			Message: fmt.Sprintf("Query execution exceeded the %s limit.", r.limits.Timeout),
		}
	case errors.Is(err, context.Canceled):
		return &Error{Code: CodeCancelled, Message: "Query execution was cancelled."}
	}

	if converted, ok := convertKnown(err); ok {
		return converted
	}

	return &Error{
		Code:    CodeExecutionError,
		Message: "Query execution failed.",
		cause:   err,
	}
}

func (r *Runner) nextRunID() string {
	return fmt.Sprintf("run-%d-%d", time.Now().UnixNano(), r.seq.Add(1))
}
