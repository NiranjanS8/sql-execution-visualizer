package validate

import (
	"errors"
	"strings"
	"testing"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

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

func resolve(t *testing.T, sql string) (*Query, error) {
	t.Helper()

	stmt, err := sqlparse.Parse(sql)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	return Resolve(stmt, sampleCatalog(t))
}

func mustResolve(t *testing.T, sql string) *Query {
	t.Helper()

	query, err := resolve(t, sql)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", sql, err)
	}
	return query
}

func resolveFailure(t *testing.T, sql string) *sqlparse.Error {
	t.Helper()

	_, err := resolve(t, sql)
	if err == nil {
		t.Fatalf("Resolve(%q) succeeded, want an error", sql)
	}

	var sqlErr *sqlparse.Error
	if !errors.As(err, &sqlErr) {
		t.Fatalf("Resolve(%q) error = %T, want *sqlparse.Error", sql, err)
	}
	return sqlErr
}

func outputNames(query *Query) []string {
	names := make([]string, len(query.Output))
	for i, column := range query.Output {
		names[i] = column.Name
	}
	return names
}

func TestResolveSimpleProjection(t *testing.T) {
	query := mustResolve(t, "SELECT name, salary FROM employees")

	if query.Source.Table.Name != "employees" {
		t.Errorf("source = %q, want employees", query.Source.Table.Name)
	}
	if got := outputNames(query); strings.Join(got, ",") != "name,salary" {
		t.Errorf("output names = %v, want name, salary", got)
	}

	first, ok := query.Output[0].Expr.(*sqlparse.ColumnRef)
	if !ok || first.Resolved == nil {
		t.Fatalf("first output is not a resolved column reference")
	}
	if first.Resolved.Index != 1 {
		t.Errorf("name index = %d, want 1", first.Resolved.Index)
	}
	if first.Resolved.Column.Type != catalog.TypeText {
		t.Errorf("name type = %v, want text", first.Resolved.Column.Type)
	}

	second := query.Output[1].Expr.(*sqlparse.ColumnRef)
	if second.Resolved.Index != 2 {
		t.Errorf("salary index = %d, want 2", second.Resolved.Index)
	}
}

func TestResolveStarExpandsInSchemaOrder(t *testing.T) {
	query := mustResolve(t, "SELECT * FROM employees")

	want := "id,name,salary,dept_id,active"
	if got := strings.Join(outputNames(query), ","); got != want {
		t.Errorf("output names = %s, want %s", got, want)
	}

	for index, column := range query.Output {
		ref, ok := column.Expr.(*sqlparse.ColumnRef)
		if !ok || ref.Resolved == nil {
			t.Fatalf("output %d is not a resolved column reference", index)
		}
		if ref.Resolved.Index != index {
			t.Errorf("output %d index = %d, want %d", index, ref.Resolved.Index, index)
		}
	}
}

func TestResolveQualifiedStar(t *testing.T) {
	query := mustResolve(t, "SELECT e.* FROM employees e")

	if len(query.Output) != 5 {
		t.Fatalf("output = %d, want 5", len(query.Output))
	}
	ref := query.Output[0].Expr.(*sqlparse.ColumnRef)
	if ref.Table != "e" {
		t.Errorf("wildcard qualifier = %q, want e", ref.Table)
	}
}

func TestResolveAliasAndOrdering(t *testing.T) {
	query := mustResolve(t, `
		SELECT e.name AS employee
		FROM employees AS e
		WHERE e.active = TRUE
		ORDER BY e.salary DESC
	`)

	if query.Source.Alias != "e" {
		t.Errorf("alias = %q, want e", query.Source.Alias)
	}
	if query.Output[0].Name != "employee" {
		t.Errorf("output name = %q, want employee", query.Output[0].Name)
	}

	where := query.Where.(*sqlparse.Binary)
	ref := where.Left.(*sqlparse.ColumnRef)
	if ref.Resolved == nil || ref.Resolved.Index != 4 {
		t.Errorf("active index = %v, want 4", ref.Resolved)
	}

	if len(query.OrderBy) != 1 || !query.OrderBy[0].Desc {
		t.Fatalf("order by = %+v, want one descending term", query.OrderBy)
	}
	ordered := query.OrderBy[0].Expr.(*sqlparse.ColumnRef)
	if ordered.Resolved == nil || ordered.Resolved.Index != 2 {
		t.Errorf("salary index = %v, want 2", ordered.Resolved)
	}
}

func TestResolveImplicitAlias(t *testing.T) {
	query := mustResolve(t, "SELECT name employee FROM employees")

	if query.Output[0].Name != "employee" {
		t.Errorf("output name = %q, want employee", query.Output[0].Name)
	}
}

func TestResolveDefaultOutputNames(t *testing.T) {
	query := mustResolve(t, "SELECT name, salary + 1 FROM employees")

	want := "name,expr_2"
	if got := strings.Join(outputNames(query), ","); got != want {
		t.Errorf("output names = %s, want %s", got, want)
	}
}

func TestResolveCaseInsensitiveIdentifiers(t *testing.T) {
	query := mustResolve(t, "SELECT NAME FROM EMPLOYEES")

	if query.Output[0].Name != "name" {
		t.Errorf("output name = %q, want the canonical name", query.Output[0].Name)
	}
}

func TestResolveQualifiedByTableName(t *testing.T) {
	query := mustResolve(t, "SELECT employees.name FROM employees")

	if query.Output[0].Name != "name" {
		t.Errorf("output name = %q, want name", query.Output[0].Name)
	}
}

func TestResolveDuplicateOutputNamesAllowed(t *testing.T) {
	query := mustResolve(t, "SELECT id, id FROM employees")

	if len(query.Output) != 2 {
		t.Fatalf("output = %d, want 2", len(query.Output))
	}
	if query.Output[0].Name != "id" || query.Output[1].Name != "id" {
		t.Errorf("output names = %v, want two id columns", outputNames(query))
	}
}

func TestResolveUnknownTable(t *testing.T) {
	err := resolveFailure(t, "SELECT * FROM nope")

	if err.Code != sqlparse.CodeUnknownTable {
		t.Errorf("code = %q, want unknown_table", err.Code)
	}
	if !strings.Contains(err.Message, `Table "nope" does not exist`) {
		t.Errorf("message = %q, want it to name the table", err.Message)
	}
	if !strings.Contains(err.Message, "Available tables: departments, employees") {
		t.Errorf("message = %q, want the available tables", err.Message)
	}
	if err.Line != 1 || err.Column != 15 {
		t.Errorf("position = %d:%d, want 1:15", err.Line, err.Column)
	}
}

func TestResolveUnknownColumn(t *testing.T) {
	err := resolveFailure(t, "SELECT nope FROM employees")

	if err.Code != sqlparse.CodeUnknownColumn {
		t.Errorf("code = %q, want unknown_column", err.Code)
	}
	if !strings.Contains(err.Message, `Column "nope" does not exist in table "employees"`) {
		t.Errorf("message = %q, want it to name the column and table", err.Message)
	}
	if !strings.Contains(err.Message, "Available columns: id, name, salary, dept_id, active") {
		t.Errorf("message = %q, want the available columns", err.Message)
	}
	if err.Line != 1 || err.Column != 8 {
		t.Errorf("position = %d:%d, want 1:8", err.Line, err.Column)
	}
}

func TestResolveUnknownQualifiedColumn(t *testing.T) {
	err := resolveFailure(t, "SELECT e.nope FROM employees e")

	if err.Code != sqlparse.CodeUnknownColumn {
		t.Errorf("code = %q, want unknown_column", err.Code)
	}
	if !strings.Contains(err.Message, `Column "nope" does not exist in table "e"`) {
		t.Errorf("message = %q, want it to name the alias", err.Message)
	}
}

func TestResolveUnknownQualifier(t *testing.T) {
	err := resolveFailure(t, "SELECT x.name FROM employees e")

	if err.Code != sqlparse.CodeUnknownTable {
		t.Errorf("code = %q, want unknown_table", err.Code)
	}
	if !strings.Contains(err.Message, `Table or alias "x" is not in this query`) {
		t.Errorf("message = %q, want it to name the qualifier", err.Message)
	}
	if !strings.Contains(err.Message, "Available names: e") {
		t.Errorf("message = %q, want the available qualifiers", err.Message)
	}
}

func TestResolveAliasHidesTableName(t *testing.T) {
	err := resolveFailure(t, "SELECT employees.name FROM employees e")

	if err.Code != sqlparse.CodeUnknownTable {
		t.Errorf("code = %q, want unknown_table", err.Code)
	}
}

func TestResolveDetectsAmbiguousColumn(t *testing.T) {
	registry := sampleCatalog(t)
	employees, _ := registry.Lookup("employees")
	departments, _ := registry.Lookup("departments")

	sources := []Source{
		{Table: employees},
		{Table: departments},
	}
	ref := &sqlparse.ColumnRef{Name: "name", Pos: sqlparse.Position{Line: 1, Column: 8}}

	_, err := bindColumn(sources, ref)
	if err == nil {
		t.Fatal("expected an ambiguity error")
	}

	var sqlErr *sqlparse.Error
	if !errors.As(err, &sqlErr) {
		t.Fatalf("error = %T, want *sqlparse.Error", err)
	}
	if sqlErr.Code != sqlparse.CodeAmbiguousColumn {
		t.Errorf("code = %q, want ambiguous_column", sqlErr.Code)
	}
	if !strings.Contains(sqlErr.Message, "qualify it with one of: employees, departments") {
		t.Errorf("message = %q, want both qualifiers", sqlErr.Message)
	}
}

func TestResolveUniqueColumnAcrossSources(t *testing.T) {
	registry := sampleCatalog(t)
	employees, _ := registry.Lookup("employees")
	departments, _ := registry.Lookup("departments")

	sources := []Source{
		{Table: employees},
		{Table: departments},
	}
	ref := &sqlparse.ColumnRef{Name: "salary", Pos: sqlparse.Position{Line: 1, Column: 8}}

	resolved, err := bindColumn(sources, ref)
	if err != nil {
		t.Fatalf("bindColumn: %v", err)
	}
	if resolved.Source.Name != "employees" || resolved.Index != 2 {
		t.Errorf("resolved = %+v, want employees.salary at index 2", resolved)
	}
}

func TestResolveOrderByPositionRejected(t *testing.T) {
	err := resolveFailure(t, "SELECT name FROM employees ORDER BY 1")

	if err.Code != sqlparse.CodeUnsupportedSyntax {
		t.Errorf("code = %q, want unsupported_syntax", err.Code)
	}
	if !strings.Contains(err.Message, "ORDER BY by position") {
		t.Errorf("message = %q, want it to explain the limitation", err.Message)
	}
}

func TestResolveOrderByOutputAliasRejected(t *testing.T) {
	err := resolveFailure(t, "SELECT salary AS pay FROM employees ORDER BY pay")

	if err.Code != sqlparse.CodeUnsupportedSyntax {
		t.Errorf("code = %q, want unsupported_syntax", err.Code)
	}
	if !strings.Contains(err.Message, `output alias "pay"`) {
		t.Errorf("message = %q, want it to name the alias", err.Message)
	}
}

func TestResolveOrderByExpression(t *testing.T) {
	query := mustResolve(t, "SELECT name FROM employees ORDER BY salary + 1 DESC")

	if len(query.OrderBy) != 1 || !query.OrderBy[0].Desc {
		t.Fatalf("order by = %+v, want one descending term", query.OrderBy)
	}
	sum := query.OrderBy[0].Expr.(*sqlparse.Binary)
	ref := sum.Left.(*sqlparse.ColumnRef)
	if ref.Resolved == nil || ref.Resolved.Index != 2 {
		t.Errorf("salary index = %v, want 2", ref.Resolved)
	}
}

func TestResolveLimit(t *testing.T) {
	query := mustResolve(t, "SELECT * FROM employees LIMIT 0")

	if query.Limit == nil || *query.Limit != 0 {
		t.Errorf("limit = %v, want 0", query.Limit)
	}

	unlimited := mustResolve(t, "SELECT * FROM employees")
	if unlimited.Limit != nil {
		t.Errorf("limit = %v, want nil", unlimited.Limit)
	}
}

func TestResolveWhereResolvesEveryReference(t *testing.T) {
	query := mustResolve(t, `
		SELECT * FROM employees
		WHERE salary > 100 AND (active = FALSE OR dept_id IS NULL)
	`)

	refs := collectRefs(query.Where)
	if len(refs) != 3 {
		t.Fatalf("references = %d, want 3", len(refs))
	}
	for _, ref := range refs {
		if ref.Resolved == nil {
			t.Errorf("column %q was not resolved", ref.Name)
		}
	}
}

func TestResolveMissingWhereAndOrderBy(t *testing.T) {
	query := mustResolve(t, "SELECT name FROM employees")

	if query.Where != nil {
		t.Errorf("where = %v, want nil", query.Where)
	}
	if query.OrderBy != nil {
		t.Errorf("order by = %v, want nil", query.OrderBy)
	}
}

func collectRefs(expr sqlparse.Expression) []*sqlparse.ColumnRef {
	switch node := expr.(type) {
	case *sqlparse.ColumnRef:
		return []*sqlparse.ColumnRef{node}
	case *sqlparse.Unary:
		return collectRefs(node.Operand)
	case *sqlparse.Binary:
		return append(collectRefs(node.Left), collectRefs(node.Right)...)
	case *sqlparse.IsNull:
		return collectRefs(node.Operand)
	default:
		return nil
	}
}
