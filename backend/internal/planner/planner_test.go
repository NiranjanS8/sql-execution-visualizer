package planner

import (
	"context"
	"strings"
	"testing"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/execution"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/validate"
)

var testPos = sqlparse.Position{Line: 1, Column: 1}

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

func buildPlan(t *testing.T, sql string) *Plan {
	t.Helper()

	stmt, err := sqlparse.Parse(sql)
	if err != nil {
		t.Fatalf("Parse(%q): %v", sql, err)
	}
	query, err := validate.Resolve(stmt, sampleCatalog(t))
	if err != nil {
		t.Fatalf("Resolve(%q): %v", sql, err)
	}
	plan, err := Build(query)
	if err != nil {
		t.Fatalf("Build(%q): %v", sql, err)
	}
	return plan
}

func runPlan(t *testing.T, plan *Plan) []catalog.Row {
	t.Helper()

	op, err := BuildOperators(plan)
	if err != nil {
		t.Fatalf("BuildOperators: %v", err)
	}
	rows, err := execution.Collect(context.Background(), op)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	return rows
}

func planKinds(root Node) []Kind {
	var kinds []Kind
	for node := root; node != nil; {
		kinds = append(kinds, node.Kind())
		children := node.Children()
		if len(children) == 0 {
			break
		}
		node = children[0]
	}
	return kinds
}

func TestBuildOrdersOperators(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []Kind
	}{
		{
			"project over scan",
			"SELECT name FROM employees",
			[]Kind{KindProject, KindScan},
		},
		{
			"filter below project",
			"SELECT name FROM employees WHERE salary > 100000",
			[]Kind{KindProject, KindFilter, KindScan},
		},
		{
			"sort below project",
			"SELECT name FROM employees ORDER BY salary",
			[]Kind{KindProject, KindSort, KindScan},
		},
		{
			"limit above project",
			"SELECT name FROM employees LIMIT 5",
			[]Kind{KindLimit, KindProject, KindScan},
		},
		{
			"every clause",
			"SELECT name FROM employees WHERE salary > 100000 ORDER BY salary DESC LIMIT 5",
			[]Kind{KindLimit, KindProject, KindSort, KindFilter, KindScan},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := buildPlan(t, test.sql)
			if got := planKinds(plan.Root); !equalKinds(got, test.want) {
				t.Errorf("plan kinds = %v, want %v\n%s", got, test.want, plan.Explain())
			}
		})
	}
}

func TestBuildProjectionNamesAndTypes(t *testing.T) {
	plan := buildPlan(t, "SELECT name AS employee, salary * 2 AS doubled, NULL FROM employees LIMIT 1")

	project, ok := plan.Root.Children()[0].(*Project)
	if !ok {
		t.Fatalf("root child = %T, want *Project", plan.Root.Children()[0])
	}

	wantNames := []string{"employee", "doubled", "expr_3"}
	wantTypes := []catalog.Type{catalog.TypeText, catalog.TypeInteger, catalog.TypeNull}
	for i := range wantNames {
		if got := project.Schema().ColumnAt(i).Name; got != wantNames[i] {
			t.Errorf("column %d name = %q, want %q", i, got, wantNames[i])
		}
		if got := project.Schema().ColumnAt(i).Type; got != wantTypes[i] {
			t.Errorf("column %d type = %s, want %s", i, got, wantTypes[i])
		}
	}
}

func TestPlanExplain(t *testing.T) {
	plan := buildPlan(t,
		"SELECT name AS employee, salary * 2 AS doubled FROM employees WHERE salary > 120000 ORDER BY salary DESC LIMIT 2")

	want := strings.Join([]string{
		"limit 2",
		"  project name AS employee, (salary * 2) AS doubled",
		"    sort salary DESC",
		"      filter (salary > 120000)",
		"        scan employees",
	}, "\n")

	if got := plan.Explain(); got != want {
		t.Errorf("Explain() =\n%s\nwant\n%s", got, want)
	}
}

func TestBuildExecutesEndToEnd(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantNames []string
		wantRows  [][]catalog.Value
	}{
		{
			"where sort and limit",
			"SELECT name, salary FROM employees WHERE salary > 120000 ORDER BY salary DESC LIMIT 3",
			[]string{"name", "salary"},
			[][]catalog.Value{
				{catalog.Text("Alan Turing"), catalog.Int(151_000)},
				{catalog.Text("Ada Lovelace"), catalog.Int(145_000)},
				{catalog.Text("Grace Hopper"), catalog.Int(132_000)},
			},
		},
		{
			"wildcard projection",
			"SELECT * FROM employees LIMIT 1",
			[]string{"id", "name", "salary", "dept_id", "active"},
			[][]catalog.Value{
				{catalog.Int(1), catalog.Text("Ada Lovelace"), catalog.Int(145_000), catalog.Int(10), catalog.Bool(true)},
			},
		},
		{
			"alias with sort on a source column",
			"SELECT name AS employee FROM employees ORDER BY salary DESC LIMIT 2",
			[]string{"employee"},
			[][]catalog.Value{
				{catalog.Text("Alan Turing")},
				{catalog.Text("Ada Lovelace")},
			},
		},
		{
			"literal projection",
			"SELECT NULL, 1 + 2 FROM employees LIMIT 1",
			[]string{"expr_1", "expr_2"},
			[][]catalog.Value{
				{catalog.Null(), catalog.Int(3)},
			},
		},
		{
			"null predicate",
			"SELECT name FROM employees WHERE salary IS NULL",
			[]string{"name"},
			[][]catalog.Value{
				{catalog.Text("Edsger Dijkstra")},
			},
		},
		{
			"table alias and qualified names",
			"SELECT e.name FROM employees AS e WHERE e.salary > 140000 ORDER BY e.name",
			[]string{"name"},
			[][]catalog.Value{
				{catalog.Text("Ada Lovelace")},
				{catalog.Text("Alan Turing")},
			},
		},
		{
			"ascending sort puts nulls last",
			"SELECT id FROM employees ORDER BY salary",
			[]string{"id"},
			[][]catalog.Value{
				{catalog.Int(8)},
				{catalog.Int(4)},
				{catalog.Int(6)},
				{catalog.Int(5)},
				{catalog.Int(2)},
				{catalog.Int(1)},
				{catalog.Int(3)},
				{catalog.Int(7)},
			},
		},
		{
			"limit zero",
			"SELECT name FROM employees LIMIT 0",
			[]string{"name"},
			nil,
		},
		{
			"empty result keeps its schema",
			"SELECT name FROM employees WHERE salary > 1000000",
			[]string{"name"},
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := buildPlan(t, test.sql)

			names := plan.Root.Schema().Names()
			if !equalStrings(names, test.wantNames) {
				t.Fatalf("schema names = %v, want %v", names, test.wantNames)
			}

			rows := runPlan(t, plan)
			if len(rows) != len(test.wantRows) {
				t.Fatalf("rows = %d, want %d\n%s", len(rows), len(test.wantRows), plan.Explain())
			}
			for i, want := range test.wantRows {
				if len(rows[i]) != len(want) {
					t.Fatalf("row %d has %d values, want %d", i, len(rows[i]), len(want))
				}
				for j := range want {
					if !rows[i][j].Equals(want[j]) {
						t.Errorf("row %d column %d = %s, want %s", i, j, rows[i][j], want[j])
					}
				}
			}
		})
	}
}

func TestPlanNodesPreserveSchemas(t *testing.T) {
	table := sampleTable(t, "employees")
	scan := NewScan(table)

	ref := &sqlparse.ColumnRef{
		Name: "name",
		Pos:  testPos,
		Resolved: &sqlparse.ResolvedColumn{
			Column: catalog.Column{Name: "name", Type: catalog.TypeText},
			Index:  1,
		},
	}
	predicate := &sqlparse.Literal{Value: catalog.Bool(true), Pos: testPos}

	if got := scan.Schema().Names(); !equalStrings(got, table.Schema.Names()) {
		t.Errorf("scan schema = %v, want %v", got, table.Schema.Names())
	}
	if got := NewFilter(scan, predicate).Schema().Names(); !equalStrings(got, table.Schema.Names()) {
		t.Errorf("filter schema = %v, want %v", got, table.Schema.Names())
	}
	if got := NewSort(scan, SortTerm{Expr: ref}).Schema().Names(); !equalStrings(got, table.Schema.Names()) {
		t.Errorf("sort schema = %v, want %v", got, table.Schema.Names())
	}
	limit, err := NewLimit(scan, 1)
	if err != nil {
		t.Fatalf("NewLimit: %v", err)
	}
	if got := limit.Schema().Names(); !equalStrings(got, table.Schema.Names()) {
		t.Errorf("limit schema = %v, want %v", got, table.Schema.Names())
	}
}

func TestNewProjectRejectsInvalidColumns(t *testing.T) {
	scan := NewScan(sampleTable(t, "employees"))

	if _, err := NewProject(scan); err == nil {
		t.Error("NewProject with no columns succeeded, want an error")
	}
	if _, err := NewProject(scan, Column{Name: "", Type: catalog.TypeText}); err == nil {
		t.Error("NewProject with an empty name succeeded, want an error")
	}
}

func TestNewLimitRejectsNegativeCount(t *testing.T) {
	if _, err := NewLimit(NewScan(sampleTable(t, "employees")), -1); err == nil {
		t.Fatal("NewLimit(-1) succeeded, want an error")
	}
}

func TestBuildRejectsNilQuery(t *testing.T) {
	if _, err := Build(nil); err == nil {
		t.Fatal("Build(nil) succeeded, want an error")
	}
	if _, err := Build(&validate.Query{}); err == nil {
		t.Fatal("Build without a source table succeeded, want an error")
	}
}

func TestFormatExpr(t *testing.T) {
	column := func(name string) *sqlparse.ColumnRef {
		return &sqlparse.ColumnRef{Name: name, Pos: testPos}
	}
	binary := func(op sqlparse.BinaryOp, left, right sqlparse.Expression) *sqlparse.Binary {
		return &sqlparse.Binary{Op: op, Left: left, Right: right, Pos: testPos}
	}

	tests := []struct {
		name string
		expr sqlparse.Expression
		want string
	}{
		{"integer literal", &sqlparse.Literal{Value: catalog.Int(1), Pos: testPos}, "1"},
		{"text literal", &sqlparse.Literal{Value: catalog.Text("a"), Pos: testPos}, "'a'"},
		{"null literal", &sqlparse.Literal{Value: catalog.Null(), Pos: testPos}, "NULL"},
		{"column", column("name"), "name"},
		{"qualified column", &sqlparse.ColumnRef{Table: "e", Name: "id", Pos: testPos}, "e.id"},
		{"star", &sqlparse.Star{Pos: testPos}, "*"},
		{"qualified star", &sqlparse.Star{Table: "e", Pos: testPos}, "e.*"},
		{"or", binary(sqlparse.OpOr, column("a"), column("b")), "(a OR b)"},
		{"and", binary(sqlparse.OpAnd, column("a"), column("b")), "(a AND b)"},
		{"equal", binary(sqlparse.OpEqual, column("a"), column("b")), "(a = b)"},
		{"not equal", binary(sqlparse.OpNotEqual, column("a"), column("b")), "(a != b)"},
		{"less", binary(sqlparse.OpLess, column("a"), column("b")), "(a < b)"},
		{"less equal", binary(sqlparse.OpLessEqual, column("a"), column("b")), "(a <= b)"},
		{"greater", binary(sqlparse.OpGreater, column("a"), column("b")), "(a > b)"},
		{"greater equal", binary(sqlparse.OpGreaterEqual, column("a"), column("b")), "(a >= b)"},
		{"add", binary(sqlparse.OpAdd, column("a"), column("b")), "(a + b)"},
		{"subtract", binary(sqlparse.OpSubtract, column("a"), column("b")), "(a - b)"},
		{"multiply", binary(sqlparse.OpMultiply, column("a"), column("b")), "(a * b)"},
		{"divide", binary(sqlparse.OpDivide, column("a"), column("b")), "(a / b)"},
		{"not", &sqlparse.Unary{Op: sqlparse.OpNot, Operand: column("a"), Pos: testPos}, "NOT a"},
		{"negate", &sqlparse.Unary{Op: sqlparse.OpNegate, Operand: column("a"), Pos: testPos}, "-a"},
		{"nested", binary(sqlparse.OpAdd, column("a"), binary(sqlparse.OpMultiply, column("b"), column("c"))), "(a + (b * c))"},
		{"is null", &sqlparse.IsNull{Operand: column("a"), Pos: testPos}, "a IS NULL"},
		{"is not null", &sqlparse.IsNull{Operand: column("a"), Negated: true, Pos: testPos}, "a IS NOT NULL"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatExpr(test.expr); got != test.want {
				t.Errorf("formatExpr = %q, want %q", got, test.want)
			}
		})
	}
}

func TestExplainShowsQualifiedColumnAlias(t *testing.T) {
	plan := buildPlan(t, "SELECT e.name FROM employees AS e LIMIT 1")

	if got := plan.Explain(); !strings.Contains(got, "project e.name AS name") {
		t.Errorf("Explain() =\n%s\nwant it to show the qualified source and output name", got)
	}
}

func TestPlanValidateRejectsBrokenNodes(t *testing.T) {
	scan := NewScan(sampleTable(t, "employees"))
	ref := &sqlparse.ColumnRef{
		Name: "name",
		Pos:  testPos,
		Resolved: &sqlparse.ResolvedColumn{
			Column: catalog.Column{Name: "name", Type: catalog.TypeText},
			Index:  1,
		},
	}
	booleanRef := &sqlparse.ColumnRef{
		Name: "active",
		Pos:  testPos,
		Resolved: &sqlparse.ResolvedColumn{
			Column: catalog.Column{Name: "active", Type: catalog.TypeBoolean},
			Index:  4,
		},
	}

	mismatched, err := catalog.NewResultSchema(catalog.ColumnDef{Name: "other", Type: catalog.TypeText})
	if err != nil {
		t.Fatalf("NewResultSchema: %v", err)
	}

	tests := []struct {
		name string
		plan *Plan
		want string
	}{
		{"nil plan", nil, "no root"},
		{"nil root", &Plan{}, "no root"},
		{"nil scan table", &Plan{Root: NewScan(nil)}, "no table"},
		{"filter without predicate", &Plan{Root: NewFilter(scan, nil)}, "no predicate"},
		{"non-boolean filter", &Plan{Root: NewFilter(scan, &sqlparse.Literal{Value: catalog.Int(1), Pos: testPos})}, "want boolean"},
		{"sort without terms", &Plan{Root: NewSort(scan)}, "no terms"},
		{"boolean sort", &Plan{Root: NewSort(scan, SortTerm{Expr: booleanRef})}, "booleans have no ordering"},
		{"project without columns", &Plan{Root: &Project{Input: scan}}, "no columns"},
		{"project schema mismatch", &Plan{Root: &Project{Input: scan, Columns: []Column{{Name: "name", Type: catalog.TypeText, Expr: ref}}, schema: mismatched}}, "projection column"},
		{"negative limit", &Plan{Root: &Limit{Input: scan, Count: -1}}, "must not be negative"},
		{"unknown node", &Plan{Root: unknownNode{}}, "unknown plan node"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.plan.Validate()
			if err == nil {
				t.Fatal("Validate succeeded, want an error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %q, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestBuildOperatorsRejectsNilPlan(t *testing.T) {
	if _, err := BuildOperators(nil); err == nil {
		t.Fatal("BuildOperators(nil) succeeded, want an error")
	}
	if _, err := BuildOperators(&Plan{}); err == nil {
		t.Fatal("BuildOperators without a root succeeded, want an error")
	}
	if _, err := BuildOperators(&Plan{Root: unknownNode{}}); err == nil {
		t.Fatal("BuildOperators with an unknown node succeeded, want an error")
	}
}

func TestExplainWithoutRoot(t *testing.T) {
	if got := (&Plan{}).Explain(); got != "" {
		t.Errorf("Explain() = %q, want an empty string", got)
	}
}

type unknownNode struct{}

func (unknownNode) Kind() Kind             { return Kind("mystery") }
func (unknownNode) Children() []Node       { return nil }
func (unknownNode) Schema() catalog.Schema { return catalog.Schema{} }
func (unknownNode) Describe() string       { return "" }

func sampleTable(t *testing.T, name string) *catalog.Table {
	t.Helper()

	for _, table := range catalog.SampleTables() {
		if table.Name == name {
			return table
		}
	}
	t.Fatalf("sample table %q is missing", name)
	return nil
}

func equalKinds(got, want []Kind) bool {
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
