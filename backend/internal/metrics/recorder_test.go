package metrics_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/execution"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/metrics"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/planner"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/validate"
)

const allClauses = "SELECT name FROM employees WHERE salary > 120000 ORDER BY salary DESC LIMIT 3"

func sampleCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()

	registry := catalog.New()
	for _, table := range catalog.SampleTables() {
		if err := registry.Create(table); err != nil {
			t.Fatalf("register %s: %v", table.Name, err)
		}
	}
	return registry
}

func buildPlan(t *testing.T, registry *catalog.Catalog, sql string) *planner.Plan {
	t.Helper()

	stmt, err := sqlparse.Parse(sql)
	if err != nil {
		t.Fatalf("Parse(%q): %v", sql, err)
	}
	query, err := validate.Resolve(stmt, registry)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", sql, err)
	}
	plan, err := planner.Build(query)
	if err != nil {
		t.Fatalf("Build(%q): %v", sql, err)
	}
	return plan
}

func execute(t *testing.T, sql string, opts metrics.Options) ([]catalog.Row, metrics.Trace) {
	t.Helper()

	plan := buildPlan(t, sampleCatalog(t), sql)
	rows, trace, err := metrics.Execute(context.Background(), plan, opts)
	if err != nil {
		t.Fatalf("Execute(%q): %v", sql, err)
	}
	return rows, trace
}

func nodeOfKind(t *testing.T, trace metrics.Trace, kind planner.Kind) metrics.Node {
	t.Helper()

	for _, node := range trace.Nodes {
		if node.Kind == kind {
			return node
		}
	}
	t.Fatalf("trace has no %s node: %+v", kind, trace.Nodes)
	return metrics.Node{}
}

func assertCounts(t *testing.T, node metrics.Node, rowsIn, rowsOut int64, status metrics.Status) {
	t.Helper()

	if node.RowsIn != rowsIn {
		t.Errorf("%s rows in = %d, want %d", node.Kind, node.RowsIn, rowsIn)
	}
	if node.RowsOut != rowsOut {
		t.Errorf("%s rows out = %d, want %d", node.Kind, node.RowsOut, rowsOut)
	}
	if node.Status != status {
		t.Errorf("%s status = %q, want %q", node.Kind, node.Status, status)
	}
}

func TestRecorderSingleNodePlan(t *testing.T) {
	registry := sampleCatalog(t)
	employees, ok := registry.Lookup("employees")
	if !ok {
		t.Fatal("employees table is missing")
	}

	plan := &planner.Plan{Root: planner.NewScan(employees)}
	rows, trace, err := metrics.Execute(context.Background(), plan, metrics.DefaultOptions())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rows) != 8 {
		t.Fatalf("rows = %d, want 8", len(rows))
	}
	if len(trace.Nodes) != 1 || len(trace.Edges) != 0 {
		t.Fatalf("nodes = %d, edges = %d, want 1 and 0", len(trace.Nodes), len(trace.Edges))
	}

	node := trace.Nodes[0]
	if node.ID != "n1" || trace.RootID != "n1" {
		t.Errorf("node id = %q, root = %q, want n1 for both", node.ID, trace.RootID)
	}
	if node.ParentID != "" {
		t.Errorf("root parent = %q, want empty", node.ParentID)
	}
	assertCounts(t, node, 0, 8, metrics.StatusCompleted)
}

func TestRecorderAssignsStablePreOrderIDs(t *testing.T) {
	registry := sampleCatalog(t)
	firstPlan := buildPlan(t, registry, allClauses)
	_, first, err := metrics.Execute(context.Background(), firstPlan, metrics.DefaultOptions())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	wantKinds := []planner.Kind{
		planner.KindLimit,
		planner.KindProject,
		planner.KindSort,
		planner.KindFilter,
		planner.KindScan,
	}
	wantLabels := []string{"3", "name", "salary DESC", "(salary > 120000)", "employees"}

	if first.RootID != "n1" {
		t.Errorf("root id = %q, want n1", first.RootID)
	}
	if len(first.Nodes) != len(wantKinds) {
		t.Fatalf("nodes = %d, want %d", len(first.Nodes), len(wantKinds))
	}
	for i, want := range wantKinds {
		node := first.Nodes[i]
		wantID := "n" + strconv.Itoa(i+1)
		if node.ID != wantID {
			t.Errorf("node %d id = %q, want %s", i, node.ID, wantID)
		}
		if node.Kind != want {
			t.Errorf("node %d kind = %q, want %q", i, node.Kind, want)
		}
		if node.Label != wantLabels[i] {
			t.Errorf("node %d label = %q, want %q", i, node.Label, wantLabels[i])
		}
		if i > 0 && node.ParentID != "n"+strconv.Itoa(i) {
			t.Errorf("node %d parent = %q, want n%d", i, node.ParentID, i)
		}
	}

	wantEdges := [][2]string{
		{"n1", "n2"},
		{"n2", "n3"},
		{"n3", "n4"},
		{"n4", "n5"},
	}
	if len(first.Edges) != len(wantEdges) {
		t.Fatalf("edges = %d, want %d", len(first.Edges), len(wantEdges))
	}
	for i, want := range wantEdges {
		if first.Edges[i].From != want[0] || first.Edges[i].To != want[1] {
			t.Errorf("edge %d = %s -> %s, want %s -> %s",
				i, first.Edges[i].From, first.Edges[i].To, want[0], want[1])
		}
	}

	secondPlan := buildPlan(t, registry, allClauses)
	_, second, err := metrics.Execute(context.Background(), secondPlan, metrics.DefaultOptions())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for i := range first.Nodes {
		if first.Nodes[i].ID != second.Nodes[i].ID {
			t.Errorf("node %d id changed between runs: %q then %q", i, first.Nodes[i].ID, second.Nodes[i].ID)
		}
		if first.Nodes[i].Kind != second.Nodes[i].Kind {
			t.Errorf("node %d kind changed between runs: %q then %q", i, first.Nodes[i].Kind, second.Nodes[i].Kind)
		}
		if first.Nodes[i].Label != second.Nodes[i].Label {
			t.Errorf("node %d label changed between runs: %q then %q", i, first.Nodes[i].Label, second.Nodes[i].Label)
		}
	}
}

func TestRecorderCountsRowsAndStatuses(t *testing.T) {
	rows, trace := execute(t, allClauses, metrics.DefaultOptions())

	if trace.Status != metrics.StatusCompleted {
		t.Errorf("trace status = %q, want %q", trace.Status, metrics.StatusCompleted)
	}
	if trace.Error != "" {
		t.Errorf("trace error = %q, want empty", trace.Error)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}

	assertCounts(t, nodeOfKind(t, trace, planner.KindScan), 0, 8, metrics.StatusCompleted)
	assertCounts(t, nodeOfKind(t, trace, planner.KindFilter), 8, 5, metrics.StatusCompleted)
	assertCounts(t, nodeOfKind(t, trace, planner.KindSort), 5, 3, metrics.StatusStopped)
	assertCounts(t, nodeOfKind(t, trace, planner.KindProject), 3, 3, metrics.StatusStopped)
	assertCounts(t, nodeOfKind(t, trace, planner.KindLimit), 3, 3, metrics.StatusCompleted)

	for _, node := range trace.Nodes {
		if node.Duration < 0 {
			t.Errorf("%s duration = %v, want non-negative", node.Kind, node.Duration)
		}
		if node.Duration > trace.Duration {
			t.Errorf("%s duration = %v, longer than the run %v", node.Kind, node.Duration, trace.Duration)
		}
	}
}

func TestRecorderLimitZeroLeavesInputsUntouched(t *testing.T) {
	rows, trace := execute(t, "SELECT name FROM employees LIMIT 0", metrics.DefaultOptions())

	if len(rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(rows))
	}
	if trace.Status != metrics.StatusCompleted {
		t.Errorf("trace status = %q, want %q", trace.Status, metrics.StatusCompleted)
	}

	assertCounts(t, nodeOfKind(t, trace, planner.KindLimit), 0, 0, metrics.StatusCompleted)
	assertCounts(t, nodeOfKind(t, trace, planner.KindProject), 0, 0, metrics.StatusNotStarted)
	assertCounts(t, nodeOfKind(t, trace, planner.KindScan), 0, 0, metrics.StatusNotStarted)
}

func TestRecorderRecordsFailures(t *testing.T) {
	plan := buildPlan(t, sampleCatalog(t), "SELECT 1 / 0 FROM employees")
	rows, trace, err := metrics.Execute(context.Background(), plan, metrics.DefaultOptions())
	if err == nil {
		t.Fatal("Execute succeeded, want a division-by-zero error")
	}
	if rows != nil {
		t.Errorf("rows = %v, want nil", rows)
	}
	if trace.Status != metrics.StatusFailed {
		t.Errorf("trace status = %q, want %q", trace.Status, metrics.StatusFailed)
	}
	if !strings.Contains(trace.Error, "Division by zero") {
		t.Errorf("trace error = %q, want it to mention division by zero", trace.Error)
	}

	project := nodeOfKind(t, trace, planner.KindProject)
	assertCounts(t, project, 1, 0, metrics.StatusFailed)
	if !strings.Contains(project.Error, "Division by zero") {
		t.Errorf("project error = %q, want it to mention division by zero", project.Error)
	}

	scan := nodeOfKind(t, trace, planner.KindScan)
	assertCounts(t, scan, 0, 1, metrics.StatusStopped)
	if scan.Error != "" {
		t.Errorf("scan error = %q, want empty", scan.Error)
	}
}

func TestRecorderRecordsCancellation(t *testing.T) {
	plan := buildPlan(t, sampleCatalog(t), "SELECT name FROM employees")

	t.Run("before the first row", func(t *testing.T) {
		recorder, err := metrics.NewRecorder(plan, metrics.DefaultOptions())
		if err != nil {
			t.Fatalf("NewRecorder: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, runErr := execution.Collect(ctx, recorder.Operator())
		trace := recorder.Finish(runErr)

		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("run error = %v, want context.Canceled", runErr)
		}
		if trace.Status != metrics.StatusCancelled {
			t.Errorf("trace status = %q, want %q", trace.Status, metrics.StatusCancelled)
		}
		assertCounts(t, nodeOfKind(t, trace, planner.KindProject), 0, 0, metrics.StatusCancelled)
		assertCounts(t, nodeOfKind(t, trace, planner.KindScan), 0, 0, metrics.StatusNotStarted)
	})

	t.Run("after the first row", func(t *testing.T) {
		recorder, err := metrics.NewRecorder(plan, metrics.DefaultOptions())
		if err != nil {
			t.Fatalf("NewRecorder: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		if _, err := recorder.Operator().Next(ctx); err != nil {
			t.Fatalf("first Next: %v", err)
		}
		cancel()

		_, runErr := recorder.Operator().Next(ctx)
		trace := recorder.Finish(runErr)

		if !errors.Is(runErr, context.Canceled) {
			t.Fatalf("run error = %v, want context.Canceled", runErr)
		}
		assertCounts(t, nodeOfKind(t, trace, planner.KindProject), 1, 1, metrics.StatusCancelled)
		assertCounts(t, nodeOfKind(t, trace, planner.KindScan), 0, 1, metrics.StatusCancelled)
	})
}

func TestRecorderBoundsSamples(t *testing.T) {
	opts := metrics.Options{SampleLimit: 2, CaptureInput: true, CaptureOutput: true}
	_, trace := execute(t, "SELECT name FROM employees", opts)

	scan := nodeOfKind(t, trace, planner.KindScan)
	if len(scan.Output.Rows) != 2 {
		t.Errorf("scan output sample = %d rows, want 2", len(scan.Output.Rows))
	}
	if !scan.Output.Truncated {
		t.Error("scan output sample is not marked truncated")
	}
	if len(scan.Input.Rows) != 0 || scan.Input.Truncated {
		t.Errorf("scan input sample = %d rows truncated=%v, want empty and not truncated",
			len(scan.Input.Rows), scan.Input.Truncated)
	}

	project := nodeOfKind(t, trace, planner.KindProject)
	if len(project.Input.Rows) != 2 {
		t.Errorf("project input sample = %d rows, want 2", len(project.Input.Rows))
	}
	if !project.Input.Truncated {
		t.Error("project input sample is not marked truncated")
	}
	if len(project.Output.Rows) != 2 {
		t.Errorf("project output sample = %d rows, want 2", len(project.Output.Rows))
	}
	if !project.Output.Truncated {
		t.Error("project output sample is not marked truncated")
	}

	if got := project.Output.Rows[0][0]; !got.Equals(catalog.Text("Ada Lovelace")) {
		t.Errorf("project output sample row 0 = %s, want 'Ada Lovelace'", got)
	}
	if got := project.Output.Rows[1][0]; !got.Equals(catalog.Text("Grace Hopper")) {
		t.Errorf("project output sample row 1 = %s, want 'Grace Hopper'", got)
	}
}

func TestRecorderSamplesAreCopies(t *testing.T) {
	registry := sampleCatalog(t)
	plan := buildPlan(t, registry, "SELECT name FROM employees")
	_, trace, err := metrics.Execute(context.Background(), plan, metrics.Options{
		SampleLimit:   1,
		CaptureOutput: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	table, ok := registry.Lookup("employees")
	if !ok {
		t.Fatal("employees table is missing")
	}
	before := table.Rows[0][0]

	scan := nodeOfKind(t, trace, planner.KindScan)
	scan.Output.Rows[0][0] = catalog.Int(999)

	if !table.Rows[0][0].Equals(before) {
		t.Errorf("mutating the sample changed the table: %s, want %s", table.Rows[0][0], before)
	}
}

func TestRecorderWithoutSamplingStillCounts(t *testing.T) {
	rows, trace := execute(t, "SELECT name FROM employees", metrics.Options{})

	if len(rows) != 8 {
		t.Fatalf("rows = %d, want 8", len(rows))
	}

	project := nodeOfKind(t, trace, planner.KindProject)
	assertCounts(t, project, 8, 8, metrics.StatusCompleted)
	if len(project.Input.Rows) != 0 || len(project.Output.Rows) != 0 {
		t.Errorf("samples = %d/%d rows, want none", len(project.Input.Rows), len(project.Output.Rows))
	}
	if !project.Input.Truncated || !project.Output.Truncated {
		t.Error("unsampled rows are not marked truncated")
	}
}

func TestRecorderSamplingCanBeOneSided(t *testing.T) {
	_, trace := execute(t, "SELECT name FROM employees", metrics.Options{
		SampleLimit:   3,
		CaptureOutput: true,
	})

	project := nodeOfKind(t, trace, planner.KindProject)
	if len(project.Input.Rows) != 0 {
		t.Errorf("project input sample = %d rows, want none", len(project.Input.Rows))
	}
	if !project.Input.Truncated {
		t.Error("project input sample is not marked truncated")
	}
	if len(project.Output.Rows) != 3 {
		t.Errorf("project output sample = %d rows, want 3", len(project.Output.Rows))
	}
}

func TestInstrumentationDoesNotChangeResults(t *testing.T) {
	tests := []string{
		allClauses,
		"SELECT * FROM employees LIMIT 1",
		"SELECT name FROM employees WHERE salary IS NULL",
		"SELECT name FROM employees WHERE salary > 1000000",
		"SELECT name FROM employees LIMIT 0",
		"SELECT id FROM employees ORDER BY salary",
	}

	for _, sql := range tests {
		t.Run(sql, func(t *testing.T) {
			plan := buildPlan(t, sampleCatalog(t), sql)

			plainOp, err := planner.BuildOperators(plan)
			if err != nil {
				t.Fatalf("BuildOperators: %v", err)
			}
			plain, err := execution.Collect(context.Background(), plainOp)
			if err != nil {
				t.Fatalf("Collect: %v", err)
			}

			instrumented, trace, err := metrics.Execute(context.Background(), plan, metrics.DefaultOptions())
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if len(instrumented) != len(plain) {
				t.Fatalf("instrumented rows = %d, want %d", len(instrumented), len(plain))
			}
			for i := range plain {
				if len(instrumented[i]) != len(plain[i]) {
					t.Fatalf("row %d has %d values, want %d", i, len(instrumented[i]), len(plain[i]))
				}
				for j := range plain[i] {
					if !instrumented[i][j].Equals(plain[i][j]) {
						t.Errorf("row %d column %d = %s, want %s", i, j, instrumented[i][j], plain[i][j])
					}
				}
			}
			if trace.Status != metrics.StatusCompleted {
				t.Errorf("trace status = %q, want %q", trace.Status, metrics.StatusCompleted)
			}
		})
	}
}

func TestRecorderGraphIsComplete(t *testing.T) {
	_, trace := execute(t, allClauses, metrics.DefaultOptions())

	if len(trace.Edges) != len(trace.Nodes)-1 {
		t.Fatalf("edges = %d, want %d for %d nodes", len(trace.Edges), len(trace.Nodes)-1, len(trace.Nodes))
	}

	parents := make(map[string]string)
	children := make(map[string][]string)
	for _, edge := range trace.Edges {
		if _, exists := parents[edge.To]; exists {
			t.Errorf("node %s has more than one parent", edge.To)
		}
		parents[edge.To] = edge.From
		children[edge.From] = append(children[edge.From], edge.To)
	}

	for _, node := range trace.Nodes {
		if node.ID == trace.RootID {
			if _, hasParent := parents[node.ID]; hasParent {
				t.Errorf("root %s has a parent", node.ID)
			}
		} else if parents[node.ID] == "" {
			t.Errorf("node %s is not reachable from the root", node.ID)
		}

		got := children[node.ID]
		if len(got) != len(node.Children) {
			t.Errorf("node %s edges = %v, want children %v", node.ID, got, node.Children)
			continue
		}
		for i := range got {
			if got[i] != node.Children[i] {
				t.Errorf("node %s edges = %v, want children %v", node.ID, got, node.Children)
				break
			}
		}
	}
}

func TestConcurrentRecordersAreIsolated(t *testing.T) {
	registry := sampleCatalog(t)
	fullPlan := buildPlan(t, registry, "SELECT name FROM employees")
	limitedPlan := buildPlan(t, registry, allClauses)

	const runs = 12
	var wg sync.WaitGroup
	failures := make(chan string, runs)

	for i := 0; i < runs; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			plan := fullPlan
			wantRows := 8
			if index%2 == 1 {
				plan = limitedPlan
				wantRows = 3
			}

			rows, trace, err := metrics.Execute(context.Background(), plan, metrics.Options{
				SampleLimit:   index%3 + 1,
				CaptureOutput: true,
			})
			if err != nil {
				failures <- "Execute: " + err.Error()
				return
			}
			if len(rows) != wantRows {
				failures <- "rows = " + strconv.Itoa(len(rows)) + ", want " + strconv.Itoa(wantRows)
				return
			}
			if trace.RootID != "n1" {
				failures <- "root id = " + trace.RootID
				return
			}
			if trace.Status != metrics.StatusCompleted {
				failures <- "status = " + string(trace.Status)
				return
			}
			if index%2 == 1 {
				if got := nodeOfKindNoFail(trace, planner.KindLimit); got == nil || got.RowsOut != 3 {
					failures <- "limit rows out are wrong"
				}
			}
		}(i)
	}

	wg.Wait()
	close(failures)

	for failure := range failures {
		t.Error(failure)
	}
}

func nodeOfKindNoFail(trace metrics.Trace, kind planner.Kind) *metrics.Node {
	for i := range trace.Nodes {
		if trace.Nodes[i].Kind == kind {
			return &trace.Nodes[i]
		}
	}
	return nil
}

func TestNewRecorderRejectsInvalidInput(t *testing.T) {
	table := sampleCatalog(t)
	employees, ok := table.Lookup("employees")
	if !ok {
		t.Fatal("employees table is missing")
	}
	scan := planner.NewScan(employees)

	tests := []struct {
		name string
		plan *planner.Plan
		opts metrics.Options
		want string
	}{
		{"nil plan", nil, metrics.DefaultOptions(), "no root"},
		{"no root", &planner.Plan{}, metrics.DefaultOptions(), "no root"},
		{"negative sample limit", &planner.Plan{Root: scan}, metrics.Options{SampleLimit: -1}, "sample limit"},
		{"invalid plan", &planner.Plan{Root: planner.NewFilter(scan, nil)}, metrics.DefaultOptions(), "no predicate"},
		{"unknown node", &planner.Plan{Root: unknownNode{}}, metrics.DefaultOptions(), "unknown plan node"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := metrics.NewRecorder(test.plan, test.opts); err == nil {
				t.Fatal("NewRecorder succeeded, want an error")
			} else if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %q, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := metrics.DefaultOptions()
	if opts.SampleLimit != 5 || !opts.CaptureInput || !opts.CaptureOutput {
		t.Errorf("DefaultOptions() = %+v, want five rows at both points", opts)
	}
}

func TestFinishIsIdempotent(t *testing.T) {
	plan := buildPlan(t, sampleCatalog(t), "SELECT name FROM employees LIMIT 2")
	recorder, err := metrics.NewRecorder(plan, metrics.DefaultOptions())
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}
	if _, err := execution.Collect(context.Background(), recorder.Operator()); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	first := recorder.Finish(nil)
	second := recorder.Finish(nil)
	if first.RootID != second.RootID || first.Status != second.Status || len(first.Nodes) != len(second.Nodes) {
		t.Errorf("Finish returned different traces: %+v and %+v", first, second)
	}
}

func TestInstrumentedOperatorKeepsSchema(t *testing.T) {
	plan := buildPlan(t, sampleCatalog(t), "SELECT name, salary FROM employees")
	recorder, err := metrics.NewRecorder(plan, metrics.DefaultOptions())
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	want := plan.Root.Schema().Names()
	got := recorder.Operator().Schema().Names()
	if len(got) != len(want) {
		t.Fatalf("schema names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("schema column %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExecuteRejectsInvalidPlan(t *testing.T) {
	rows, trace, err := metrics.Execute(context.Background(), nil, metrics.DefaultOptions())
	if err == nil {
		t.Fatal("Execute(nil) succeeded, want an error")
	}
	if rows != nil {
		t.Errorf("rows = %v, want nil", rows)
	}
	if trace.RootID != "" || len(trace.Nodes) != 0 {
		t.Errorf("trace = %+v, want the zero value", trace)
	}
}

func TestFinishRecordsErrorBeforeExecution(t *testing.T) {
	plan := buildPlan(t, sampleCatalog(t), "SELECT name FROM employees")
	recorder, err := metrics.NewRecorder(plan, metrics.DefaultOptions())
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	trace := recorder.Finish(errors.New("boom"))
	if trace.Status != metrics.StatusFailed {
		t.Errorf("trace status = %q, want %q", trace.Status, metrics.StatusFailed)
	}
	if !strings.Contains(trace.Error, "boom") {
		t.Errorf("trace error = %q, want it to mention boom", trace.Error)
	}

	root := nodeOfKind(t, trace, planner.KindProject)
	if root.Status != metrics.StatusFailed {
		t.Errorf("root status = %q, want %q", root.Status, metrics.StatusFailed)
	}
	if !strings.Contains(root.Error, "boom") {
		t.Errorf("root error = %q, want it to mention boom", root.Error)
	}
	scan := nodeOfKind(t, trace, planner.KindScan)
	if scan.Status != metrics.StatusNotStarted {
		t.Errorf("scan status = %q, want %q", scan.Status, metrics.StatusNotStarted)
	}
}

func TestFinishRecordsCancellationBeforeExecution(t *testing.T) {
	plan := buildPlan(t, sampleCatalog(t), "SELECT name FROM employees")
	recorder, err := metrics.NewRecorder(plan, metrics.DefaultOptions())
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	trace := recorder.Finish(context.Canceled)
	if trace.Status != metrics.StatusCancelled {
		t.Errorf("trace status = %q, want %q", trace.Status, metrics.StatusCancelled)
	}
	if !strings.Contains(trace.Error, context.Canceled.Error()) {
		t.Errorf("trace error = %q, want it to mention cancellation", trace.Error)
	}
}

type unknownNode struct{}

func (unknownNode) Kind() planner.Kind       { return planner.Kind("mystery") }
func (unknownNode) Children() []planner.Node { return nil }
func (unknownNode) Schema() catalog.Schema   { return catalog.Schema{} }
func (unknownNode) Describe() string         { return "" }
