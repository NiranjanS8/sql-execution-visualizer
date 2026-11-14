package execution

import (
	"context"
	"testing"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/validate"
)

// TestSingleTablePipeline assembles the operator chain a planner will build in
// Phase 6: scan, filter, sort, project, limit. ORDER BY resolves source columns,
// so sorting runs before projection.
func TestSingleTablePipeline(t *testing.T) {
	stmt, err := sqlparse.Parse(
		`SELECT name, salary FROM employees WHERE salary > 120000 ORDER BY salary DESC LIMIT 3`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	query, err := validate.Resolve(stmt, catalog.NewWithSamples())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	op, err := buildPipeline(query)
	if err != nil {
		t.Fatalf("buildPipeline: %v", err)
	}

	rows, err := Collect(context.Background(), op)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	want := []struct {
		name   string
		salary int64
	}{
		{"Alan Turing", 151_000},
		{"Ada Lovelace", 145_000},
		{"Grace Hopper", 132_000},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d", len(rows), len(want))
	}
	for i, expected := range want {
		if !rows[i][0].Equals(catalog.Text(expected.name)) {
			t.Errorf("row %d name = %s, want %q", i, rows[i][0], expected.name)
		}
		if !rows[i][1].Equals(catalog.Int(expected.salary)) {
			t.Errorf("row %d salary = %s, want %d", i, rows[i][1], expected.salary)
		}
	}

	if got := op.Schema().Names(); !equalStrings(got, []string{"name", "salary"}) {
		t.Errorf("result schema = %v, want [name salary]", got)
	}
}

func buildPipeline(query *validate.Query) (Operator, error) {
	var op Operator = NewScan(query.Source.Table)

	if query.Where != nil {
		op = NewFilter(op, query.Where)
	}

	if len(query.OrderBy) > 0 {
		terms := make([]SortTerm, len(query.OrderBy))
		for i, item := range query.OrderBy {
			terms[i] = SortTerm{Expr: item.Expr, Desc: item.Desc}
		}
		sorted, err := NewSort(op, terms)
		if err != nil {
			return nil, err
		}
		op = sorted
	}

	projections := make([]Projection, len(query.Output))
	for i, output := range query.Output {
		projections[i] = Projection{Name: output.Name, Type: output.Type, Expr: output.Expr}
	}
	projected, err := NewProject(op, projections)
	if err != nil {
		return nil, err
	}
	op = projected

	if query.Limit != nil {
		limited, err := NewLimit(op, *query.Limit)
		if err != nil {
			return nil, err
		}
		op = limited
	}

	return op, nil
}
