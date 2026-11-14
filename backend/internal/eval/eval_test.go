package eval

import (
	"errors"
	"strings"
	"testing"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

var testPos = sqlparse.Position{Line: 1, Column: 1}

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

func unary(op sqlparse.UnaryOp, operand sqlparse.Expression) *sqlparse.Unary {
	return &sqlparse.Unary{Op: op, Operand: operand, Pos: testPos}
}

func isNull(operand sqlparse.Expression, negated bool) *sqlparse.IsNull {
	return &sqlparse.IsNull{Operand: operand, Negated: negated, Pos: testPos}
}

func testRow() catalog.Row {
	return catalog.Row{
		catalog.Int(10),
		catalog.Decimal(2.5),
		catalog.Text("apple"),
		catalog.Bool(true),
		catalog.Null(),
	}
}

func TestInfer(t *testing.T) {
	tests := []struct {
		name string
		expr sqlparse.Expression
		want catalog.Type
	}{
		{"integer literal", lit(catalog.Int(1)), catalog.TypeInteger},
		{"decimal literal", lit(catalog.Decimal(1.5)), catalog.TypeDecimal},
		{"text literal", lit(catalog.Text("x")), catalog.TypeText},
		{"boolean literal", lit(catalog.Bool(true)), catalog.TypeBoolean},
		{"null literal", lit(catalog.Null()), catalog.TypeNull},

		{"integer column", column(0, "id", catalog.TypeInteger), catalog.TypeInteger},
		{"decimal column", column(1, "salary", catalog.TypeDecimal), catalog.TypeDecimal},

		{"integer addition", bin(sqlparse.OpAdd, lit(catalog.Int(1)), lit(catalog.Int(2))), catalog.TypeInteger},
		{"mixed addition", bin(sqlparse.OpAdd, lit(catalog.Int(1)), lit(catalog.Decimal(2.5))), catalog.TypeDecimal},
		{"null plus integer", bin(sqlparse.OpAdd, lit(catalog.Null()), lit(catalog.Int(2))), catalog.TypeInteger},
		{"integer plus null", bin(sqlparse.OpAdd, lit(catalog.Int(2)), lit(catalog.Null())), catalog.TypeInteger},
		{"null plus null", bin(sqlparse.OpAdd, lit(catalog.Null()), lit(catalog.Null())), catalog.TypeNull},

		{"unary minus", unary(sqlparse.OpNegate, lit(catalog.Int(1))), catalog.TypeInteger},
		{"not", unary(sqlparse.OpNot, lit(catalog.Bool(true))), catalog.TypeBoolean},
		{"comparison", bin(sqlparse.OpEqual, lit(catalog.Int(1)), lit(catalog.Int(2))), catalog.TypeBoolean},
		{"mixed comparison", bin(sqlparse.OpLess, lit(catalog.Int(1)), lit(catalog.Decimal(2.5))), catalog.TypeBoolean},
		{"null comparison", bin(sqlparse.OpEqual, lit(catalog.Null()), lit(catalog.Text("x"))), catalog.TypeBoolean},
		{"is null", isNull(column(0, "id", catalog.TypeInteger), false), catalog.TypeBoolean},
		{"and", bin(sqlparse.OpAnd, lit(catalog.Bool(true)), lit(catalog.Null())), catalog.TypeBoolean},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Infer(test.expr)
			if err != nil {
				t.Fatalf("Infer: %v", err)
			}
			if got != test.want {
				t.Errorf("Infer = %s, want %s", got, test.want)
			}
		})
	}
}

func TestInferRejectsInvalidOperands(t *testing.T) {
	tests := []struct {
		name string
		expr sqlparse.Expression
		want string
	}{
		{"not integer", unary(sqlparse.OpNot, lit(catalog.Int(1))), "NOT requires a boolean operand"},
		{"negate text", unary(sqlparse.OpNegate, lit(catalog.Text("x"))), "Unary - requires a numeric operand"},
		{"text addition", bin(sqlparse.OpAdd, lit(catalog.Text("x")), lit(catalog.Int(1))), "requires numeric operands, found text on the left"},
		{"boolean addition", bin(sqlparse.OpAdd, lit(catalog.Int(1)), lit(catalog.Bool(true))), "requires numeric operands, found boolean on the right"},
		{"null plus text", bin(sqlparse.OpAdd, lit(catalog.Null()), lit(catalog.Text("x"))), "requires numeric operands, found text on the right"},
		{"text equals integer", bin(sqlparse.OpEqual, lit(catalog.Text("x")), lit(catalog.Int(1))), "Cannot compare text with integer"},
		{"boolean ordering", bin(sqlparse.OpLess, lit(catalog.Bool(true)), lit(catalog.Bool(false))), "no boolean ordering"},
		{"and integer", bin(sqlparse.OpAnd, lit(catalog.Int(1)), lit(catalog.Bool(true))), "AND requires boolean operands"},
		{"or integer", bin(sqlparse.OpOr, lit(catalog.Bool(true)), lit(catalog.Int(1))), "OR requires boolean operands"},
		{"wildcard", &sqlparse.Star{Pos: testPos}, "wildcard cannot be evaluated"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := inferFailure(t, test.expr)
			if err.Code != CodeTypeError {
				t.Errorf("code = %q, want type_error", err.Code)
			}
			if !strings.Contains(err.Message, test.want) {
				t.Errorf("message = %q, want it to contain %q", err.Message, test.want)
			}
		})
	}
}

func TestInferRequiresBoundColumns(t *testing.T) {
	ref := &sqlparse.ColumnRef{Name: "name", Pos: testPos}
	err := inferFailure(t, ref)

	if err.Code != CodeExecutionError {
		t.Errorf("code = %q, want execution_error", err.Code)
	}
	if !strings.Contains(err.Message, `Column "name" is not bound`) {
		t.Errorf("message = %q, want it to explain the missing binding", err.Message)
	}
}

func TestEvalLiteralsAndColumns(t *testing.T) {
	row := testRow()

	tests := []struct {
		name string
		expr sqlparse.Expression
		want catalog.Value
	}{
		{"integer literal", lit(catalog.Int(42)), catalog.Int(42)},
		{"text literal", lit(catalog.Text("x")), catalog.Text("x")},
		{"null literal", lit(catalog.Null()), catalog.Null()},
		{"integer column", column(0, "id", catalog.TypeInteger), catalog.Int(10)},
		{"decimal column", column(1, "salary", catalog.TypeDecimal), catalog.Decimal(2.5)},
		{"text column", column(2, "name", catalog.TypeText), catalog.Text("apple")},
		{"boolean column", column(3, "active", catalog.TypeBoolean), catalog.Bool(true)},
		{"null column", column(4, "note", catalog.TypeText), catalog.Null()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Eval(test.expr, row)
			if err != nil {
				t.Fatalf("Eval: %v", err)
			}
			if !got.Equals(test.want) {
				t.Errorf("Eval = %s, want %s", got, test.want)
			}
		})
	}
}

func TestEvalArithmetic(t *testing.T) {
	row := testRow()

	tests := []struct {
		name string
		expr sqlparse.Expression
		want catalog.Value
	}{
		{"integer addition", bin(sqlparse.OpAdd, lit(catalog.Int(1)), lit(catalog.Int(2))), catalog.Int(3)},
		{"integer subtraction", bin(sqlparse.OpSubtract, lit(catalog.Int(7)), lit(catalog.Int(10))), catalog.Int(-3)},
		{"integer multiplication", bin(sqlparse.OpMultiply, lit(catalog.Int(6)), lit(catalog.Int(7))), catalog.Int(42)},
		{"integer division truncates", bin(sqlparse.OpDivide, lit(catalog.Int(7)), lit(catalog.Int(2))), catalog.Int(3)},
		{"negative integer division truncates toward zero", bin(sqlparse.OpDivide, lit(catalog.Int(-7)), lit(catalog.Int(2))), catalog.Int(-3)},
		{"mixed addition", bin(sqlparse.OpAdd, lit(catalog.Int(1)), lit(catalog.Decimal(2.5))), catalog.Decimal(3.5)},
		{"mixed division", bin(sqlparse.OpDivide, lit(catalog.Int(1)), lit(catalog.Decimal(2))), catalog.Decimal(0.5)},
		{"decimal multiplication", bin(sqlparse.OpMultiply, lit(catalog.Decimal(2.5)), lit(catalog.Int(2))), catalog.Decimal(5)},
		{"column arithmetic", bin(sqlparse.OpAdd, column(0, "id", catalog.TypeInteger), lit(catalog.Int(5))), catalog.Int(15)},
		{"decimal column arithmetic", bin(sqlparse.OpMultiply, column(1, "salary", catalog.TypeDecimal), lit(catalog.Int(2))), catalog.Decimal(5)},
		{"null plus integer", bin(sqlparse.OpAdd, lit(catalog.Null()), lit(catalog.Int(1))), catalog.Null()},
		{"integer plus null", bin(sqlparse.OpAdd, lit(catalog.Int(1)), lit(catalog.Null())), catalog.Null()},
		{"null plus null", bin(sqlparse.OpAdd, lit(catalog.Null()), lit(catalog.Null())), catalog.Null()},
		{"unary minus", unary(sqlparse.OpNegate, lit(catalog.Int(5))), catalog.Int(-5)},
		{"unary minus decimal", unary(sqlparse.OpNegate, lit(catalog.Decimal(1.5))), catalog.Decimal(-1.5)},
		{"unary minus null", unary(sqlparse.OpNegate, lit(catalog.Null())), catalog.Null()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Eval(test.expr, row)
			if err != nil {
				t.Fatalf("Eval: %v", err)
			}
			if !got.Equals(test.want) {
				t.Errorf("Eval = %s, want %s", got, test.want)
			}
		})
	}
}

func TestEvalArithmeticErrors(t *testing.T) {
	tests := []struct {
		name string
		expr sqlparse.Expression
		code ErrorCode
		want string
	}{
		{"integer division by zero", bin(sqlparse.OpDivide, lit(catalog.Int(1)), lit(catalog.Int(0))), CodeExecutionError, "Division by zero"},
		{"decimal division by zero", bin(sqlparse.OpDivide, lit(catalog.Decimal(1)), lit(catalog.Decimal(0))), CodeExecutionError, "Division by zero"},
		{"zero divided by zero", bin(sqlparse.OpDivide, lit(catalog.Decimal(0)), lit(catalog.Decimal(0))), CodeExecutionError, "Division by zero"},
		{"integer addition overflow", bin(sqlparse.OpAdd, lit(catalog.Int(9223372036854775807)), lit(catalog.Int(1))), CodeExecutionError, "addition overflows"},
		{"integer subtraction overflow", bin(sqlparse.OpSubtract, lit(catalog.Int(-9223372036854775808)), lit(catalog.Int(1))), CodeExecutionError, "subtraction overflows"},
		{"integer multiplication overflow", bin(sqlparse.OpMultiply, lit(catalog.Int(9223372036854775807)), lit(catalog.Int(2))), CodeExecutionError, "multiplication overflows"},
		{"integer multiplication overflow by minus one", bin(sqlparse.OpMultiply, lit(catalog.Int(-1)), lit(catalog.Int(-9223372036854775808))), CodeExecutionError, "multiplication overflows"},
		{"integer multiplication overflow by minus one reversed", bin(sqlparse.OpMultiply, lit(catalog.Int(-9223372036854775808)), lit(catalog.Int(-1))), CodeExecutionError, "multiplication overflows"},
		{"integer division overflow", bin(sqlparse.OpDivide, lit(catalog.Int(-9223372036854775808)), lit(catalog.Int(-1))), CodeExecutionError, "division overflows"},
		{"integer negation overflow", unary(sqlparse.OpNegate, lit(catalog.Int(-9223372036854775808))), CodeExecutionError, "Negating"},
		{"decimal overflow", bin(sqlparse.OpMultiply, lit(catalog.Decimal(1e308)), lit(catalog.Decimal(10))), CodeExecutionError, "out of range"},
		{"text arithmetic", bin(sqlparse.OpAdd, lit(catalog.Text("x")), lit(catalog.Int(1))), CodeTypeError, "requires numeric operands"},
		{"boolean arithmetic", bin(sqlparse.OpAdd, lit(catalog.Int(1)), lit(catalog.Bool(true))), CodeTypeError, "requires numeric operands"},
		{"null plus text", bin(sqlparse.OpAdd, lit(catalog.Null()), lit(catalog.Text("x"))), CodeTypeError, "requires numeric operands"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Eval(test.expr, testRow())
			if err == nil {
				t.Fatal("Eval succeeded, want an error")
			}

			var evalErr *Error
			if !errors.As(err, &evalErr) {
				t.Fatalf("error = %T, want *Error", err)
			}
			if evalErr.Code != test.code {
				t.Errorf("code = %q, want %q", evalErr.Code, test.code)
			}
			if !strings.Contains(evalErr.Message, test.want) {
				t.Errorf("message = %q, want it to contain %q", evalErr.Message, test.want)
			}
		})
	}
}

func TestEvalComparisons(t *testing.T) {
	tests := []struct {
		name string
		expr sqlparse.Expression
		want catalog.Value
	}{
		{"equal integers", bin(sqlparse.OpEqual, lit(catalog.Int(1)), lit(catalog.Int(1))), catalog.Bool(true)},
		{"unequal integers", bin(sqlparse.OpNotEqual, lit(catalog.Int(1)), lit(catalog.Int(2))), catalog.Bool(true)},
		{"less than", bin(sqlparse.OpLess, lit(catalog.Int(1)), lit(catalog.Int(2))), catalog.Bool(true)},
		{"less than or equal", bin(sqlparse.OpLessEqual, lit(catalog.Int(2)), lit(catalog.Int(2))), catalog.Bool(true)},
		{"greater than", bin(sqlparse.OpGreater, lit(catalog.Int(2)), lit(catalog.Int(1))), catalog.Bool(true)},
		{"greater than or equal", bin(sqlparse.OpGreaterEqual, lit(catalog.Int(1)), lit(catalog.Int(2))), catalog.Bool(false)},
		{"mixed numeric equality", bin(sqlparse.OpEqual, lit(catalog.Int(1)), lit(catalog.Decimal(1))), catalog.Bool(true)},
		{"mixed numeric ordering", bin(sqlparse.OpLess, lit(catalog.Int(2)), lit(catalog.Decimal(2.5))), catalog.Bool(true)},
		{"text equality", bin(sqlparse.OpEqual, lit(catalog.Text("a")), lit(catalog.Text("a"))), catalog.Bool(true)},
		{"text ordering is byte order", bin(sqlparse.OpLess, lit(catalog.Text("B")), lit(catalog.Text("a"))), catalog.Bool(true)},
		{"boolean equality", bin(sqlparse.OpEqual, lit(catalog.Bool(true)), lit(catalog.Bool(true))), catalog.Bool(true)},
		{"boolean inequality", bin(sqlparse.OpNotEqual, lit(catalog.Bool(true)), lit(catalog.Bool(false))), catalog.Bool(true)},
		{"null equality", bin(sqlparse.OpEqual, lit(catalog.Null()), lit(catalog.Int(1))), catalog.Null()},
		{"integer equals null", bin(sqlparse.OpEqual, lit(catalog.Int(1)), lit(catalog.Null())), catalog.Null()},
		{"null equals null", bin(sqlparse.OpEqual, lit(catalog.Null()), lit(catalog.Null())), catalog.Null()},
		{"column comparison", bin(sqlparse.OpGreater, column(0, "id", catalog.TypeInteger), lit(catalog.Int(5))), catalog.Bool(true)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Eval(test.expr, testRow())
			if err != nil {
				t.Fatalf("Eval: %v", err)
			}
			if !got.Equals(test.want) {
				t.Errorf("Eval = %s, want %s", got, test.want)
			}
		})
	}
}

func TestEvalComparisonErrors(t *testing.T) {
	tests := []struct {
		name string
		expr sqlparse.Expression
		want string
	}{
		{"text equals integer", bin(sqlparse.OpEqual, lit(catalog.Text("x")), lit(catalog.Int(1))), "Cannot compare text with integer"},
		{"boolean equals integer", bin(sqlparse.OpEqual, lit(catalog.Bool(true)), lit(catalog.Int(1))), "Cannot compare boolean with integer"},
		{"boolean ordering", bin(sqlparse.OpLess, lit(catalog.Bool(true)), lit(catalog.Bool(false))), "no boolean ordering"},
		{"text ordering against boolean", bin(sqlparse.OpGreater, lit(catalog.Text("x")), lit(catalog.Bool(true))), "Cannot compare text with boolean"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Eval(test.expr, testRow())
			if err == nil {
				t.Fatal("Eval succeeded, want an error")
			}

			var evalErr *Error
			if !errors.As(err, &evalErr) {
				t.Fatalf("error = %T, want *Error", err)
			}
			if evalErr.Code != CodeTypeError {
				t.Errorf("code = %q, want type_error", evalErr.Code)
			}
			if !strings.Contains(evalErr.Message, test.want) {
				t.Errorf("message = %q, want it to contain %q", evalErr.Message, test.want)
			}
		})
	}
}

func TestEvalThreeValuedLogic(t *testing.T) {
	type truth struct {
		name  string
		value catalog.Value
	}

	values := []truth{
		{"true", catalog.Bool(true)},
		{"false", catalog.Bool(false)},
		{"null", catalog.Null()},
	}

	and := map[[2]string]catalog.Value{
		{"true", "true"}:   catalog.Bool(true),
		{"true", "false"}:  catalog.Bool(false),
		{"true", "null"}:   catalog.Null(),
		{"false", "true"}:  catalog.Bool(false),
		{"false", "false"}: catalog.Bool(false),
		{"false", "null"}:  catalog.Bool(false),
		{"null", "true"}:   catalog.Null(),
		{"null", "false"}:  catalog.Bool(false),
		{"null", "null"}:   catalog.Null(),
	}

	or := map[[2]string]catalog.Value{
		{"true", "true"}:   catalog.Bool(true),
		{"true", "false"}:  catalog.Bool(true),
		{"true", "null"}:   catalog.Bool(true),
		{"false", "true"}:  catalog.Bool(true),
		{"false", "false"}: catalog.Bool(false),
		{"false", "null"}:  catalog.Null(),
		{"null", "true"}:   catalog.Bool(true),
		{"null", "false"}:  catalog.Null(),
		{"null", "null"}:   catalog.Null(),
	}

	for _, left := range values {
		for _, right := range values {
			key := [2]string{left.name, right.name}

			t.Run("AND "+left.name+" "+right.name, func(t *testing.T) {
				got, err := Eval(bin(sqlparse.OpAnd, lit(left.value), lit(right.value)), nil)
				if err != nil {
					t.Fatalf("Eval: %v", err)
				}
				if !got.Equals(and[key]) {
					t.Errorf("Eval = %s, want %s", got, and[key])
				}
			})

			t.Run("OR "+left.name+" "+right.name, func(t *testing.T) {
				got, err := Eval(bin(sqlparse.OpOr, lit(left.value), lit(right.value)), nil)
				if err != nil {
					t.Fatalf("Eval: %v", err)
				}
				if !got.Equals(or[key]) {
					t.Errorf("Eval = %s, want %s", got, or[key])
				}
			})
		}
	}

	not := map[string]catalog.Value{
		"true":  catalog.Bool(false),
		"false": catalog.Bool(true),
		"null":  catalog.Null(),
	}
	for _, value := range values {
		t.Run("NOT "+value.name, func(t *testing.T) {
			got, err := Eval(unary(sqlparse.OpNot, lit(value.value)), nil)
			if err != nil {
				t.Fatalf("Eval: %v", err)
			}
			if !got.Equals(not[value.name]) {
				t.Errorf("Eval = %s, want %s", got, not[value.name])
			}
		})
	}
}

func TestEvalBooleanOperandsMustBeBoolean(t *testing.T) {
	tests := []struct {
		name string
		expr sqlparse.Expression
		want string
	}{
		{"and left", bin(sqlparse.OpAnd, lit(catalog.Int(1)), lit(catalog.Bool(true))), "AND requires boolean operands"},
		{"and right", bin(sqlparse.OpAnd, lit(catalog.Bool(true)), lit(catalog.Int(1))), "AND requires boolean operands"},
		{"or left", bin(sqlparse.OpOr, lit(catalog.Text("x")), lit(catalog.Bool(true))), "OR requires boolean operands"},
		{"or right", bin(sqlparse.OpOr, lit(catalog.Bool(false)), lit(catalog.Text("x"))), "OR requires boolean operands"},
		{"not", unary(sqlparse.OpNot, lit(catalog.Int(1))), "NOT requires a boolean operand"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Eval(test.expr, nil)
			if err == nil {
				t.Fatal("Eval succeeded, want an error")
			}

			var evalErr *Error
			if !errors.As(err, &evalErr) {
				t.Fatalf("error = %T, want *Error", err)
			}
			if evalErr.Code != CodeTypeError {
				t.Errorf("code = %q, want type_error", evalErr.Code)
			}
			if !strings.Contains(evalErr.Message, test.want) {
				t.Errorf("message = %q, want it to contain %q", evalErr.Message, test.want)
			}
		})
	}
}

func TestEvalShortCircuits(t *testing.T) {
	divisionByZero := bin(sqlparse.OpDivide, lit(catalog.Int(1)), lit(catalog.Int(0)))
	broken := bin(sqlparse.OpEqual, divisionByZero, lit(catalog.Int(1)))

	tests := []struct {
		name    string
		expr    sqlparse.Expression
		want    catalog.Value
		wantErr bool
	}{
		{"false and error", bin(sqlparse.OpAnd, lit(catalog.Bool(false)), broken), catalog.Bool(false), false},
		{"true and error", bin(sqlparse.OpAnd, lit(catalog.Bool(true)), broken), catalog.Null(), true},
		{"null and error", bin(sqlparse.OpAnd, lit(catalog.Null()), broken), catalog.Null(), true},
		{"true or error", bin(sqlparse.OpOr, lit(catalog.Bool(true)), broken), catalog.Bool(true), false},
		{"false or error", bin(sqlparse.OpOr, lit(catalog.Bool(false)), broken), catalog.Null(), true},
		{"null or error", bin(sqlparse.OpOr, lit(catalog.Null()), broken), catalog.Null(), true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Eval(test.expr, nil)
			if test.wantErr {
				if err == nil {
					t.Fatal("Eval succeeded, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Eval: %v", err)
			}
			if !got.Equals(test.want) {
				t.Errorf("Eval = %s, want %s", got, test.want)
			}
		})
	}
}

func TestEvalIsNull(t *testing.T) {
	tests := []struct {
		name string
		expr sqlparse.Expression
		want catalog.Value
	}{
		{"null is null", isNull(lit(catalog.Null()), false), catalog.Bool(true)},
		{"null is not null", isNull(lit(catalog.Null()), true), catalog.Bool(false)},
		{"value is null", isNull(lit(catalog.Int(1)), false), catalog.Bool(false)},
		{"value is not null", isNull(lit(catalog.Int(1)), true), catalog.Bool(true)},
		{"null column is null", isNull(column(4, "note", catalog.TypeText), false), catalog.Bool(true)},
		{"expression is null", isNull(bin(sqlparse.OpAdd, lit(catalog.Null()), lit(catalog.Int(1))), false), catalog.Bool(true)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Eval(test.expr, testRow())
			if err != nil {
				t.Fatalf("Eval: %v", err)
			}
			if !got.Equals(test.want) {
				t.Errorf("Eval = %s, want %s", got, test.want)
			}
		})
	}
}

func TestEvalReportsSourcePositions(t *testing.T) {
	pos := sqlparse.Position{Line: 3, Column: 7}
	expr := &sqlparse.Binary{
		Op:    sqlparse.OpDivide,
		Left:  &sqlparse.Literal{Value: catalog.Int(1), Pos: pos},
		Right: &sqlparse.Literal{Value: catalog.Int(0), Pos: pos},
		Pos:   pos,
	}

	_, err := Eval(expr, nil)
	if err == nil {
		t.Fatal("Eval succeeded, want an error")
	}

	var evalErr *Error
	if !errors.As(err, &evalErr) {
		t.Fatalf("error = %T, want *Error", err)
	}
	if evalErr.Line != 3 || evalErr.Column != 7 {
		t.Errorf("position = %d:%d, want 3:7", evalErr.Line, evalErr.Column)
	}
	if !strings.Contains(evalErr.Error(), "line 3, column 7") {
		t.Errorf("Error() = %q, want it to include the position", evalErr.Error())
	}
}

func TestEvalRejectsMisuse(t *testing.T) {
	row := testRow()

	t.Run("unbound column", func(t *testing.T) {
		ref := &sqlparse.ColumnRef{Name: "name", Pos: testPos}
		_, err := Eval(ref, row)
		if err == nil {
			t.Fatal("Eval succeeded, want an error")
		}
		if !strings.Contains(err.Error(), `Column "name" is not bound`) {
			t.Errorf("error = %q, want it to explain the missing binding", err)
		}
	})

	t.Run("row too short", func(t *testing.T) {
		_, err := Eval(column(9, "missing", catalog.TypeInteger), row)
		if err == nil {
			t.Fatal("Eval succeeded, want an error")
		}
		var evalErr *Error
		if !errors.As(err, &evalErr) {
			t.Fatalf("error = %T, want *Error", err)
		}
		if evalErr.Code != CodeExecutionError {
			t.Errorf("code = %q, want execution_error", evalErr.Code)
		}
	})

	t.Run("wildcard", func(t *testing.T) {
		_, err := Eval(&sqlparse.Star{Pos: testPos}, row)
		if err == nil {
			t.Fatal("Eval succeeded, want an error")
		}
		var evalErr *Error
		if !errors.As(err, &evalErr) {
			t.Fatalf("error = %T, want *Error", err)
		}
		if evalErr.Code != CodeTypeError {
			t.Errorf("code = %q, want type_error", evalErr.Code)
		}
	})

	t.Run("nil expression", func(t *testing.T) {
		if _, err := Eval(nil, row); err == nil {
			t.Error("Eval(nil) succeeded, want an error")
		}
		if _, err := Infer(nil); err == nil {
			t.Error("Infer(nil) succeeded, want an error")
		}
	})
}

func inferFailure(t *testing.T, expr sqlparse.Expression) *Error {
	t.Helper()

	_, err := Infer(expr)
	if err == nil {
		t.Fatal("Infer succeeded, want an error")
	}

	var evalErr *Error
	if !errors.As(err, &evalErr) {
		t.Fatalf("error = %T, want *Error", err)
	}
	return evalErr
}
