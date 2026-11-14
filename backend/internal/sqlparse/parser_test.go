package sqlparse

import (
	"errors"
	"strings"
	"testing"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
)

func mustParse(t *testing.T, sql string) *Select {
	t.Helper()

	stmt, err := Parse(sql)
	if err != nil {
		t.Fatalf("Parse(%q): %v", sql, err)
	}
	return stmt
}

func parseFailure(t *testing.T, sql string) *Error {
	t.Helper()

	_, err := Parse(sql)
	if err == nil {
		t.Fatalf("Parse(%q) succeeded, want an error", sql)
	}

	var sqlErr *Error
	if !errors.As(err, &sqlErr) {
		t.Fatalf("Parse(%q) error = %T, want *Error", sql, err)
	}
	return sqlErr
}

func asBinary(t *testing.T, expr Expression) *Binary {
	t.Helper()

	binary, ok := expr.(*Binary)
	if !ok {
		t.Fatalf("expression = %T, want *Binary", expr)
	}
	return binary
}

func TestParseFullSelect(t *testing.T) {
	stmt := mustParse(t, `
		SELECT e.name AS employee, e.salary * 2, e.active
		FROM employees AS e
		WHERE e.salary > 100000 AND e.active = TRUE
		ORDER BY e.salary DESC, e.name
		LIMIT 25
	`)

	if len(stmt.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(stmt.Items))
	}
	if stmt.Items[0].Alias != "employee" {
		t.Errorf("first alias = %q, want employee", stmt.Items[0].Alias)
	}
	if stmt.Items[1].Alias != "" {
		t.Errorf("second alias = %q, want none", stmt.Items[1].Alias)
	}

	ref, ok := stmt.Items[0].Expr.(*ColumnRef)
	if !ok || ref.Table != "e" || ref.Name != "name" {
		t.Errorf("first item = %#v, want e.name", stmt.Items[0].Expr)
	}

	product := asBinary(t, stmt.Items[1].Expr)
	if product.Op != OpMultiply {
		t.Errorf("second item op = %v, want multiply", product.Op)
	}

	if stmt.From.Name != "employees" || stmt.From.Alias != "e" {
		t.Errorf("from = %+v, want employees as e", stmt.From)
	}

	where := asBinary(t, stmt.Where)
	if where.Op != OpAnd {
		t.Fatalf("where op = %v, want AND", where.Op)
	}
	if asBinary(t, where.Left).Op != OpGreater {
		t.Error("left side of WHERE is not a greater-than comparison")
	}
	if asBinary(t, where.Right).Op != OpEqual {
		t.Error("right side of WHERE is not an equality comparison")
	}

	if len(stmt.OrderBy) != 2 {
		t.Fatalf("order by terms = %d, want 2", len(stmt.OrderBy))
	}
	if !stmt.OrderBy[0].Desc {
		t.Error("first ORDER BY term should be descending")
	}
	if stmt.OrderBy[1].Desc {
		t.Error("second ORDER BY term should be ascending by default")
	}

	if stmt.Limit == nil || *stmt.Limit != 25 {
		t.Errorf("limit = %v, want 25", stmt.Limit)
	}
}

func TestParseSelectStar(t *testing.T) {
	stmt := mustParse(t, "SELECT * FROM employees")
	star, ok := stmt.Items[0].Expr.(*Star)
	if !ok || star.Table != "" {
		t.Errorf("item = %#v, want an unqualified star", stmt.Items[0].Expr)
	}

	qualified := mustParse(t, "SELECT e.* FROM employees e")
	star, ok = qualified.Items[0].Expr.(*Star)
	if !ok || star.Table != "e" {
		t.Errorf("item = %#v, want e.*", qualified.Items[0].Expr)
	}
}

func TestParseOperatorPrecedence(t *testing.T) {
	stmt := mustParse(t, "SELECT a + b * c FROM t")
	sum := asBinary(t, stmt.Items[0].Expr)
	if sum.Op != OpAdd {
		t.Fatalf("top op = %v, want add", sum.Op)
	}
	if asBinary(t, sum.Right).Op != OpMultiply {
		t.Error("multiplication should bind tighter than addition")
	}

	notStmt := mustParse(t, "SELECT NOT a = 1 FROM t")
	not, ok := notStmt.Items[0].Expr.(*Unary)
	if !ok || not.Op != OpNot {
		t.Fatalf("expression = %#v, want NOT", notStmt.Items[0].Expr)
	}
	if asBinary(t, not.Operand).Op != OpEqual {
		t.Error("NOT should apply to the whole comparison")
	}

	boolean := mustParse(t, "SELECT a = 1 AND b = 2 OR c = 3 FROM t")
	or := asBinary(t, boolean.Items[0].Expr)
	if or.Op != OpOr {
		t.Fatalf("top op = %v, want OR", or.Op)
	}
	if asBinary(t, or.Left).Op != OpAnd {
		t.Error("AND should bind tighter than OR")
	}

	subtraction := mustParse(t, "SELECT a - b - c FROM t")
	outer := asBinary(t, subtraction.Items[0].Expr)
	if outer.Op != OpSubtract {
		t.Fatalf("top op = %v, want subtract", outer.Op)
	}
	if asBinary(t, outer.Left).Op != OpSubtract {
		t.Error("subtraction should be left-associative")
	}
}

func TestParseLiterals(t *testing.T) {
	stmt := mustParse(t, "SELECT 'it''s', 42, 3.5, TRUE, false, NULL FROM t")

	want := []catalog.Value{
		catalog.Text("it's"),
		catalog.Int(42),
		catalog.Decimal(3.5),
		catalog.Bool(true),
		catalog.Bool(false),
		catalog.Null(),
	}

	for i, expected := range want {
		literal, ok := stmt.Items[i].Expr.(*Literal)
		if !ok {
			t.Fatalf("item %d = %T, want *Literal", i, stmt.Items[i].Expr)
		}
		if !literal.Value.Equals(expected) {
			t.Errorf("item %d = %v, want %v", i, literal.Value, expected)
		}
	}
}

func TestParseUnaryMinus(t *testing.T) {
	stmt := mustParse(t, "SELECT -5 FROM t")

	unary, ok := stmt.Items[0].Expr.(*Unary)
	if !ok || unary.Op != OpNegate {
		t.Fatalf("expression = %#v, want unary minus", stmt.Items[0].Expr)
	}
	literal, ok := unary.Operand.(*Literal)
	if !ok || !literal.Value.Equals(catalog.Int(5)) {
		t.Errorf("operand = %#v, want 5", unary.Operand)
	}
}

func TestParseIsNull(t *testing.T) {
	stmt := mustParse(t, "SELECT * FROM t WHERE a IS NULL AND b IS NOT NULL")

	where := asBinary(t, stmt.Where)
	left, ok := where.Left.(*IsNull)
	if !ok || left.Negated {
		t.Errorf("left = %#v, want IS NULL", where.Left)
	}
	right, ok := where.Right.(*IsNull)
	if !ok || !right.Negated {
		t.Errorf("right = %#v, want IS NOT NULL", where.Right)
	}
}

func TestParseComments(t *testing.T) {
	stmt := mustParse(t, `
		-- leading comment
		SELECT /* inline */ name
		FROM employees -- trailing comment
	`)

	if len(stmt.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(stmt.Items))
	}
	if stmt.From.Name != "employees" {
		t.Errorf("from = %q, want employees", stmt.From.Name)
	}
}

func TestParseQuotedIdentifiers(t *testing.T) {
	stmt := mustParse(t, `SELECT "select", "a""b" FROM "from"`)

	first, ok := stmt.Items[0].Expr.(*ColumnRef)
	if !ok || first.Name != "select" {
		t.Errorf("first item = %#v, want a quoted select", stmt.Items[0].Expr)
	}
	second, ok := stmt.Items[1].Expr.(*ColumnRef)
	if !ok || second.Name != `a"b` {
		t.Errorf("second item = %#v, want a\"b", stmt.Items[1].Expr)
	}
	if stmt.From.Name != "from" {
		t.Errorf("from = %q, want a quoted from", stmt.From.Name)
	}
}

func TestParseCaseInsensitiveKeywords(t *testing.T) {
	stmt := mustParse(t, "select Name from Employees where Name is not null order by Name desc limit 1")

	if stmt.From.Name != "Employees" {
		t.Errorf("from = %q, want Employees", stmt.From.Name)
	}
	if len(stmt.OrderBy) != 1 || !stmt.OrderBy[0].Desc {
		t.Errorf("order by = %+v, want one descending term", stmt.OrderBy)
	}
	if stmt.Limit == nil || *stmt.Limit != 1 {
		t.Errorf("limit = %v, want 1", stmt.Limit)
	}
}

func TestParseTrailingSemicolon(t *testing.T) {
	stmt := mustParse(t, "SELECT name FROM employees;")
	if stmt.From.Name != "employees" {
		t.Errorf("from = %q, want employees", stmt.From.Name)
	}
}

func TestParseImplicitAlias(t *testing.T) {
	stmt := mustParse(t, "SELECT name employee, salary * 2 pay FROM employees")

	if stmt.Items[0].Alias != "employee" {
		t.Errorf("first alias = %q, want employee", stmt.Items[0].Alias)
	}
	if stmt.Items[1].Alias != "pay" {
		t.Errorf("second alias = %q, want pay", stmt.Items[1].Alias)
	}
}

func TestParsePositions(t *testing.T) {
	stmt := mustParse(t, "SELECT a\nFROM employees")

	if stmt.Pos.Line != 1 || stmt.Pos.Column != 1 {
		t.Errorf("statement position = %+v, want 1:1", stmt.Pos)
	}
	if stmt.From.Pos.Line != 2 || stmt.From.Pos.Column != 6 {
		t.Errorf("FROM position = %+v, want line 2, column 6", stmt.From.Pos)
	}
}

func TestParseRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"empty", "", "SQL is empty"},
		{"whitespace", "  \n\t", "SQL is empty"},
		{"truncated select", "SELECT", "Unexpected end of input"},
		{"missing table", "SELECT * FROM", "Expected a table name after FROM"},
		{"missing predicate", "SELECT * FROM t WHERE", "Unexpected end of input"},
		{"trailing comma", "SELECT a, FROM t", "Expected a select item after"},
		{"two semicolons", "SELECT * FROM t;;", "Only one statement is supported"},
		{"two statements", "SELECT * FROM t; SELECT * FROM t", "Only one statement is supported"},
		{"unterminated string", "SELECT 'x FROM t", "Unterminated string literal"},
		{"unterminated identifier", `SELECT "x FROM t`, "Unterminated quoted identifier"},
		{"unterminated comment", "SELECT /* x FROM t", "Unterminated block comment"},
		{"integer overflow", "SELECT 99999999999999999999 FROM t", "out of range for a 64-bit integer"},
		{"decimal overflow", "SELECT 1e999 FROM t", "out of range"},
		{"bare exclamation", "SELECT a ! b FROM t", `Unexpected "!"`},
		{"bare colon", "SELECT a : b FROM t", `Unexpected ":"`},
		{"three part name", "SELECT a.b.c FROM t", "at most one qualifier"},
		{"chained comparison", "SELECT a = b = c FROM t", "cannot be chained"},
		{"negative limit", "SELECT * FROM t LIMIT -1", "LIMIT expects a non-negative integer"},
		{"empty order by", "SELECT * FROM t ORDER BY", "Unexpected end of input"},
		{"reserved column", "SELECT order FROM t", "reserved word"},
		{"unexpected token", "SELECT * FROM t 42", "Only one statement is supported"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := parseFailure(t, test.sql)
			if err.Code != CodeParseError {
				t.Errorf("code = %q, want parse_error", err.Code)
			}
			if !strings.Contains(err.Message, test.want) {
				t.Errorf("message = %q, want it to contain %q", err.Message, test.want)
			}
		})
	}
}

func TestParseReportsPositions(t *testing.T) {
	err := parseFailure(t, "SELECT a\nFROM t\nWHERE b = 1 GROUP BY b")

	if err.Code != CodeUnsupportedSyntax {
		t.Fatalf("code = %q, want unsupported_syntax", err.Code)
	}
	if err.Line != 3 || err.Column != 13 {
		t.Errorf("position = %d:%d, want 3:13", err.Line, err.Column)
	}
}

func TestParseRejectsUnsupportedSyntax(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"distinct", "SELECT DISTINCT a FROM t", "DISTINCT"},
		{"join", "SELECT * FROM t JOIN u ON t.id = u.id", "Joins are not supported"},
		{"group by", "SELECT * FROM t GROUP BY a", "GROUP BY"},
		{"having", "SELECT * FROM t HAVING a > 1", "HAVING"},
		{"union", "SELECT * FROM t UNION SELECT * FROM u", "Set operations"},
		{"cte", "WITH x AS (SELECT 1) SELECT * FROM x", "Common table expressions"},
		{"case", "SELECT CASE WHEN a THEN 1 END FROM t", "CASE"},
		{"function", "SELECT COUNT(*) FROM t", `Function "COUNT" is not supported`},
		{"subquery in from", "SELECT * FROM (SELECT * FROM t) x", "Subqueries are not supported"},
		{"subquery in where", "SELECT * FROM t WHERE a = (SELECT 1)", "Subqueries are not supported"},
		{"in", "SELECT * FROM t WHERE a IN (1, 2)", "IN is not supported"},
		{"between", "SELECT * FROM t WHERE a BETWEEN 1 AND 2", "BETWEEN"},
		{"like", "SELECT * FROM t WHERE a LIKE 'x'", "LIKE"},
		{"cast", "SELECT CAST(a AS INTEGER) FROM t", "CAST"},
		{"cast operator", "SELECT a::int FROM t", "casts (::)"},
		{"backtick", "SELECT `a` FROM t", "Backtick identifiers"},
		{"offset", "SELECT * FROM t OFFSET 5", "OFFSET"},
		{"limit with offset", "SELECT * FROM t LIMIT 1, 5", "LIMIT with an offset"},
		{"unary plus", "SELECT +1 FROM t", "Unary +"},
		{"insert", "INSERT INTO t VALUES (1)", "Only SELECT statements"},
		{"is true", "SELECT * FROM t WHERE a IS TRUE", "IS is only supported with NULL"},
		{"nulls first", "SELECT * FROM t ORDER BY a NULLS FIRST", "NULLS FIRST"},
		{"select without from", "SELECT 1", "SELECT without FROM"},
		{"select all", "SELECT ALL a FROM t", "SELECT ALL"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := parseFailure(t, test.sql)
			if err.Code != CodeUnsupportedSyntax {
				t.Errorf("code = %q, want unsupported_syntax", err.Code)
			}
			if !strings.Contains(err.Message, test.want) {
				t.Errorf("message = %q, want it to contain %q", err.Message, test.want)
			}
		})
	}
}
