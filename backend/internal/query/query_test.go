package query

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/metrics"
)

func sampleTables() *catalog.Catalog {
	return catalog.NewWithSamples()
}

func requireQueryError(t *testing.T, err error, want Code) *Error {
	t.Helper()

	var queryErr *Error
	if !errors.As(err, &queryErr) {
		t.Fatalf("error = %v, want *query.Error", err)
	}
	if queryErr.Code != want {
		t.Fatalf("code = %q, want %q (message: %s)", queryErr.Code, want, queryErr.Message)
	}
	return queryErr
}

func TestExecuteReturnsRowsColumnsAndTrace(t *testing.T) {
	runner := NewRunner(DefaultLimits())

	result, err := runner.Execute(context.Background(),
		"SELECT name FROM employees WHERE salary > 120000 ORDER BY salary DESC LIMIT 3",
		sampleTables())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if result.Status != metrics.StatusCompleted {
		t.Errorf("status = %q, want completed", result.Status)
	}
	if result.RunID == "" {
		t.Error("run ID is empty")
	}
	if result.Duration < 0 {
		t.Errorf("duration = %v, want non-negative", result.Duration)
	}

	if len(result.Columns) != 1 {
		t.Fatalf("columns = %d, want 1", len(result.Columns))
	}
	if result.Columns[0].ID != "c1" || result.Columns[0].Name != "name" || result.Columns[0].Type != catalog.TypeText {
		t.Errorf("column = %+v, want c1 name text", result.Columns[0])
	}

	want := []string{"Alan Turing", "Ada Lovelace", "Grace Hopper"}
	if len(result.Rows) != len(want) {
		t.Fatalf("rows = %d, want %d", len(result.Rows), len(want))
	}
	for i, name := range want {
		got, ok := result.Rows[i][0].AsText()
		if !ok || got != name {
			t.Errorf("row %d = %v, want %q", i, result.Rows[i][0], name)
		}
	}

	if result.Trace.RootID != "n1" {
		t.Errorf("root ID = %q, want n1", result.Trace.RootID)
	}
	if len(result.Trace.Nodes) != 5 {
		t.Fatalf("trace nodes = %d, want 5", len(result.Trace.Nodes))
	}
	if len(result.Trace.Edges) != 4 {
		t.Errorf("trace edges = %d, want 4", len(result.Trace.Edges))
	}
	if result.Trace.Status != metrics.StatusCompleted {
		t.Errorf("trace status = %q, want completed", result.Trace.Status)
	}
}

func TestExecuteSamplesAreBoundedAndCopied(t *testing.T) {
	runner := NewRunner(DefaultLimits())

	result, err := runner.Execute(context.Background(),
		"SELECT * FROM employees", sampleTables())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	root := result.Trace.Nodes[0]
	if len(root.Output.Rows) != 5 {
		t.Errorf("root output sample = %d rows, want the 5-row sample limit", len(root.Output.Rows))
	}
	if !root.Output.Truncated {
		t.Error("root output sample is not truncated, want truncated")
	}

	var scan *metrics.Node
	for i := range result.Trace.Nodes {
		if result.Trace.Nodes[i].Kind == "scan" {
			scan = &result.Trace.Nodes[i]
			break
		}
	}
	if scan == nil {
		t.Fatal("trace has no scan node")
	}
	if len(scan.Output.Rows) != 5 {
		t.Errorf("scan output sample = %d rows, want the 5-row sample limit", len(scan.Output.Rows))
	}
	if !scan.Output.Truncated {
		t.Error("scan output sample is not truncated, want truncated")
	}
}

func TestExecuteReportsParseError(t *testing.T) {
	_, err := NewRunner(DefaultLimits()).Execute(context.Background(),
		"SELECT FROM employees", sampleTables())

	queryErr := requireQueryError(t, err, CodeParseError)
	if queryErr.Line == 0 || queryErr.Column == 0 {
		t.Errorf("location = %d:%d, want a source position", queryErr.Line, queryErr.Column)
	}
}

func TestExecuteReportsResolutionErrors(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		code Code
	}{
		{name: "unknown table", sql: "SELECT * FROM nope", code: CodeUnknownTable},
		{name: "unknown column", sql: "SELECT nope FROM employees", code: CodeUnknownColumn},
		{name: "type error", sql: "SELECT name FROM employees WHERE salary", code: CodeTypeError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRunner(DefaultLimits()).Execute(context.Background(), test.sql, sampleTables())
			requireQueryError(t, err, test.code)
		})
	}
}

func TestExecuteReportsExecutionError(t *testing.T) {
	_, err := NewRunner(DefaultLimits()).Execute(context.Background(),
		"SELECT salary / 0 FROM employees", sampleTables())

	queryErr := requireQueryError(t, err, CodeExecutionError)
	if !strings.Contains(queryErr.Message, "Division by zero") {
		t.Errorf("message = %q, want a division-by-zero explanation", queryErr.Message)
	}
	if queryErr.Line == 0 {
		t.Error("line = 0, want a source position")
	}
}

func TestExecuteEnforcesSQLLength(t *testing.T) {
	runner := NewRunner(Limits{MaxSQLLength: 10, Timeout: time.Second, MaxRows: 10})

	_, err := runner.Execute(context.Background(), "SELECT * FROM employees", sampleTables())
	queryErr := requireQueryError(t, err, CodeInvalidRequest)
	if !strings.Contains(queryErr.Message, "limit is 10") {
		t.Errorf("message = %q, want the configured limit", queryErr.Message)
	}
}

func TestExecuteEnforcesResultLimit(t *testing.T) {
	runner := NewRunner(Limits{MaxSQLLength: 10_000, Timeout: time.Second, MaxRows: 3})

	_, err := runner.Execute(context.Background(), "SELECT * FROM employees", sampleTables())
	queryErr := requireQueryError(t, err, CodeResultLimit)
	if !strings.Contains(queryErr.Message, "3 rows") {
		t.Errorf("message = %q, want the configured row limit", queryErr.Message)
	}
}

func TestExecuteAllowsExactlyTheResultLimit(t *testing.T) {
	runner := NewRunner(Limits{MaxSQLLength: 10_000, Timeout: time.Second, MaxRows: 8})

	result, err := runner.Execute(context.Background(), "SELECT * FROM employees", sampleTables())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Rows) != 8 {
		t.Errorf("rows = %d, want 8", len(result.Rows))
	}
}

func TestExecuteWithLimitZeroReturnsNoRows(t *testing.T) {
	result, err := NewRunner(DefaultLimits()).Execute(context.Background(),
		"SELECT * FROM employees LIMIT 0", sampleTables())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("rows = %d, want 0", len(result.Rows))
	}
	if result.Status != metrics.StatusCompleted {
		t.Errorf("status = %q, want completed", result.Status)
	}
}

func TestExecuteReportsTimeout(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := NewRunner(DefaultLimits()).Execute(ctx, "SELECT * FROM employees", sampleTables())
	requireQueryError(t, err, CodeTimeout)
}

func TestExecuteReportsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewRunner(DefaultLimits()).Execute(ctx, "SELECT * FROM employees", sampleTables())
	requireQueryError(t, err, CodeCancelled)
}

func TestRunIDsAreUnique(t *testing.T) {
	runner := NewRunner(DefaultLimits())
	tables := sampleTables()

	first, err := runner.Execute(context.Background(), "SELECT * FROM employees LIMIT 1", tables)
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	second, err := runner.Execute(context.Background(), "SELECT * FROM employees LIMIT 1", tables)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if first.RunID == second.RunID {
		t.Errorf("run IDs collide: %q", first.RunID)
	}
}

func TestNewRunnerAppliesDefaults(t *testing.T) {
	runner := NewRunner(Limits{Samples: metrics.Options{SampleLimit: -1}})
	limits := runner.Limits()
	defaults := DefaultLimits()

	if limits.MaxSQLLength != defaults.MaxSQLLength {
		t.Errorf("max SQL length = %d, want %d", limits.MaxSQLLength, defaults.MaxSQLLength)
	}
	if limits.Timeout != defaults.Timeout {
		t.Errorf("timeout = %v, want %v", limits.Timeout, defaults.Timeout)
	}
	if limits.MaxRows != defaults.MaxRows {
		t.Errorf("max rows = %d, want %d", limits.MaxRows, defaults.MaxRows)
	}
	if limits.Samples.SampleLimit != 0 {
		t.Errorf("sample limit = %d, want 0", limits.Samples.SampleLimit)
	}
}
