package execution

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/eval"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

var testPos = sqlparse.Position{Line: 1, Column: 1}

func employees(t *testing.T) *catalog.Table {
	t.Helper()

	for _, table := range catalog.SampleTables() {
		if table.Name == "employees" {
			return table
		}
	}
	t.Fatal("sample table employees is missing")
	return nil
}

func lit(value catalog.Value) *sqlparse.Literal {
	return &sqlparse.Literal{Value: value, Pos: testPos}
}

func column(index int, name string, typ catalog.Type) *sqlparse.ColumnRef {
	return &sqlparse.ColumnRef{
		Name: name,
		Pos:  sqlparse.Position{Line: 2, Column: 3},
		Resolved: &sqlparse.ResolvedColumn{
			Column: catalog.Column{Name: name, Type: typ},
			Index:  index,
		},
	}
}

func bin(op sqlparse.BinaryOp, left, right sqlparse.Expression) *sqlparse.Binary {
	return &sqlparse.Binary{Op: op, Left: left, Right: right, Pos: testPos}
}

func collectIDs(t *testing.T, op Operator) []int64 {
	t.Helper()

	rows, err := Collect(context.Background(), op)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	ids := make([]int64, len(rows))
	for i, row := range rows {
		id, ok := row[0].AsInt()
		if !ok {
			t.Fatalf("row %d id = %s, want an integer", i, row[0])
		}
		ids[i] = id
	}
	return ids
}

func equalIDs(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestScanEmitsRowsInStoredOrder(t *testing.T) {
	table := employees(t)
	scan := NewScan(table)

	if got := scan.Schema().Names(); !equalStrings(got, table.Schema.Names()) {
		t.Errorf("schema names = %v, want %v", got, table.Schema.Names())
	}

	ids := collectIDs(t, scan)
	want := []int64{1, 2, 3, 4, 5, 6, 7, 8}
	if !equalIDs(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}

func TestScanReturnsEOFAfterLastRow(t *testing.T) {
	scan := NewScan(employees(t))
	ctx := context.Background()

	for {
		_, err := scan.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
	}

	if _, err := scan.Next(ctx); !errors.Is(err, io.EOF) {
		t.Errorf("Next after EOF = %v, want io.EOF", err)
	}
}

func TestScanHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scan := NewScan(employees(t))
	if _, err := scan.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Next = %v, want context.Canceled", err)
	}
}

func TestFilterKeepsOnlyTrueRows(t *testing.T) {
	predicate := bin(sqlparse.OpGreater, column(2, "salary", catalog.TypeInteger), lit(catalog.Int(120_000)))
	filter := NewFilter(NewScan(employees(t)), predicate)

	if got := filter.Schema().Names(); !equalStrings(got, employees(t).Schema.Names()) {
		t.Errorf("filter changed the schema: %v", got)
	}

	ids := collectIDs(t, filter)
	want := []int64{1, 2, 3, 5, 6}
	if !equalIDs(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}

func TestFilterDropsNullPredicates(t *testing.T) {
	filter := NewFilter(NewScan(employees(t)), lit(catalog.Null()))

	ids := collectIDs(t, filter)
	if len(ids) != 0 {
		t.Errorf("ids = %v, want no rows for an unknown predicate", ids)
	}
}

func TestFilterKeepsRowsMatchingIsNull(t *testing.T) {
	predicate := &sqlparse.IsNull{
		Operand: column(2, "salary", catalog.TypeInteger),
		Pos:     testPos,
	}
	filter := NewFilter(NewScan(employees(t)), predicate)

	ids := collectIDs(t, filter)
	want := []int64{7}
	if !equalIDs(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}

func TestFilterRejectsNonBooleanPredicate(t *testing.T) {
	filter := NewFilter(NewScan(employees(t)), lit(catalog.Int(1)))

	_, err := Collect(context.Background(), filter)
	if err == nil {
		t.Fatal("Collect succeeded, want a type error")
	}

	var evalErr *eval.Error
	if !errors.As(err, &evalErr) {
		t.Fatalf("error = %T, want *eval.Error", err)
	}
	if evalErr.Code != eval.CodeTypeError {
		t.Errorf("code = %q, want type_error", evalErr.Code)
	}
	if !strings.Contains(evalErr.Message, "WHERE requires a boolean predicate") {
		t.Errorf("message = %q, want it to explain the predicate requirement", evalErr.Message)
	}
}

func TestProjectBuildsResultSchemaAndRows(t *testing.T) {
	project, err := NewProject(NewScan(employees(t)), []Projection{
		{Name: "name", Type: catalog.TypeText, Expr: column(1, "name", catalog.TypeText)},
		{Name: "raised", Type: catalog.TypeInteger, Expr: bin(sqlparse.OpAdd, column(2, "salary", catalog.TypeInteger), lit(catalog.Int(1_000)))},
	})
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	if got := project.Schema().Names(); !equalStrings(got, []string{"name", "raised"}) {
		t.Errorf("schema names = %v, want [name raised]", got)
	}
	if got := project.Schema().ColumnAt(1).Type; got != catalog.TypeInteger {
		t.Errorf("raised type = %s, want integer", got)
	}

	rows, err := Collect(context.Background(), project)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(rows) != 8 {
		t.Fatalf("rows = %d, want 8", len(rows))
	}
	if !rows[0][0].Equals(catalog.Text("Ada Lovelace")) {
		t.Errorf("first name = %s", rows[0][0])
	}
	if !rows[0][1].Equals(catalog.Int(146_000)) {
		t.Errorf("first raised salary = %s, want 146000", rows[0][1])
	}
}

func TestProjectAllowsNullTypeAndDisplayNames(t *testing.T) {
	project, err := NewProject(NewScan(employees(t)), []Projection{
		{Name: "constant", Type: catalog.TypeNull, Expr: lit(catalog.Null())},
	})
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	column := project.Schema().ColumnAt(0)
	if column.Type != catalog.TypeNull {
		t.Errorf("type = %s, want null", column.Type)
	}

	row, err := project.Next(context.Background())
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if !row[0].IsNull() {
		t.Errorf("value = %s, want NULL", row[0])
	}
}

func TestProjectRejectsEmptyProjection(t *testing.T) {
	if _, err := NewProject(NewScan(employees(t)), nil); err == nil {
		t.Fatal("NewProject(nil) succeeded, want an error")
	}
}

func TestProjectPropagatesEvaluationErrors(t *testing.T) {
	project, err := NewProject(NewScan(employees(t)), []Projection{
		{Name: "broken", Type: catalog.TypeInteger, Expr: bin(sqlparse.OpDivide, lit(catalog.Int(1)), lit(catalog.Int(0)))},
	})
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	if _, err := project.Next(context.Background()); err == nil {
		t.Fatal("Next succeeded, want an execution error")
	}
}

func TestLimitStopsAfterCount(t *testing.T) {
	counted := &countingOperator{input: NewScan(employees(t))}
	limit, err := NewLimit(counted, 3)
	if err != nil {
		t.Fatalf("NewLimit: %v", err)
	}

	ids := collectIDs(t, limit)
	if want := []int64{1, 2, 3}; !equalIDs(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	if counted.pulled != 3 {
		t.Errorf("pulled = %d, want 3; limit should not read past its count", counted.pulled)
	}
}

func TestLimitZeroEmitsNothing(t *testing.T) {
	counted := &countingOperator{input: NewScan(employees(t))}
	limit, err := NewLimit(counted, 0)
	if err != nil {
		t.Fatalf("NewLimit: %v", err)
	}

	rows, err := Collect(context.Background(), limit)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0", len(rows))
	}
	if counted.pulled != 0 {
		t.Errorf("pulled = %d, want 0; LIMIT 0 must not read its input", counted.pulled)
	}
}

func TestLimitAboveInputSizeEmitsAllRows(t *testing.T) {
	limit, err := NewLimit(NewScan(employees(t)), 100)
	if err != nil {
		t.Fatalf("NewLimit: %v", err)
	}

	if rows, err := Collect(context.Background(), limit); err != nil || len(rows) != 8 {
		t.Fatalf("Collect = %d rows, %v; want 8 rows, nil", len(rows), err)
	}
}

func TestLimitRejectsNegativeCount(t *testing.T) {
	if _, err := NewLimit(NewScan(employees(t)), -1); err == nil {
		t.Fatal("NewLimit(-1) succeeded, want an error")
	}
}

func TestSortAscendingPutsNullsLast(t *testing.T) {
	sorted := mustSort(t, SortTerm{Expr: column(2, "salary", catalog.TypeInteger)})

	if got := sorted.Schema().Names(); !equalStrings(got, employees(t).Schema.Names()) {
		t.Errorf("sort changed the schema: %v", got)
	}

	ids := collectIDs(t, sorted)
	want := []int64{8, 4, 6, 5, 2, 1, 3, 7}
	if !equalIDs(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}

func TestSortDescendingPutsNullsLast(t *testing.T) {
	sorted := mustSort(t, SortTerm{Expr: column(2, "salary", catalog.TypeInteger), Desc: true})

	ids := collectIDs(t, sorted)
	want := []int64{3, 1, 2, 5, 6, 4, 8, 7}
	if !equalIDs(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}

func TestSortIsStableForEqualKeys(t *testing.T) {
	sorted := mustSort(t, SortTerm{Expr: column(3, "dept_id", catalog.TypeInteger)})

	ids := collectIDs(t, sorted)
	want := []int64{1, 2, 5, 6, 3, 4, 7, 8}
	if !equalIDs(ids, want) {
		t.Errorf("ids = %v, want %v; equal keys must keep input order", ids, want)
	}
}

func TestSortUsesEveryTermInOrder(t *testing.T) {
	sorted := mustSort(t,
		SortTerm{Expr: column(3, "dept_id", catalog.TypeInteger)},
		SortTerm{Expr: column(1, "name", catalog.TypeText)},
	)

	ids := collectIDs(t, sorted)
	want := []int64{1, 5, 2, 6, 3, 7, 4, 8}
	if !equalIDs(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}

func TestSortRejectsBooleanTerms(t *testing.T) {
	_, err := NewSort(NewScan(employees(t)), []SortTerm{
		{Expr: column(4, "active", catalog.TypeBoolean)},
	})
	if err == nil {
		t.Fatal("NewSort succeeded, want a type error")
	}

	var evalErr *eval.Error
	if !errors.As(err, &evalErr) {
		t.Fatalf("error = %T, want *eval.Error", err)
	}
	if evalErr.Code != eval.CodeTypeError {
		t.Errorf("code = %q, want type_error", evalErr.Code)
	}
	if !strings.Contains(evalErr.Message, "booleans have no ordering") {
		t.Errorf("message = %q, want it to explain the missing ordering", evalErr.Message)
	}
}

func TestSortWithoutTermsReturnsInput(t *testing.T) {
	scan := NewScan(employees(t))
	sorted, err := NewSort(scan, nil)
	if err != nil {
		t.Fatalf("NewSort: %v", err)
	}
	if sorted != scan {
		t.Error("NewSort with no terms wrapped the input instead of returning it")
	}
}

func TestSortHonoursCancellation(t *testing.T) {
	sorted := mustSort(t, SortTerm{Expr: column(2, "salary", catalog.TypeInteger)})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := sorted.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Next = %v, want context.Canceled", err)
	}
	if _, err := sorted.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("second Next = %v, want the cached context.Canceled", err)
	}
}

func TestSortPropagatesKeyEvaluationErrors(t *testing.T) {
	sorted, err := NewSort(NewScan(employees(t)), []SortTerm{
		{Expr: bin(sqlparse.OpDivide, lit(catalog.Int(1)), lit(catalog.Int(0)))},
	})
	if err != nil {
		t.Fatalf("NewSort: %v", err)
	}

	if _, err := sorted.Next(context.Background()); err == nil {
		t.Fatal("Next succeeded, want an execution error")
	}
}

func mustSort(t *testing.T, terms ...SortTerm) Operator {
	t.Helper()

	sorted, err := NewSort(NewScan(employees(t)), terms)
	if err != nil {
		t.Fatalf("NewSort: %v", err)
	}
	return sorted
}

type countingOperator struct {
	input  Operator
	pulled int
}

func (c *countingOperator) Schema() catalog.Schema { return c.input.Schema() }

func (c *countingOperator) Next(ctx context.Context) (catalog.Row, error) {
	row, err := c.input.Next(ctx)
	if err == nil {
		c.pulled++
	}
	return row, err
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
