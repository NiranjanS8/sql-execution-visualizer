package eval

import (
	"cmp"
	"fmt"
	"math"
	"strings"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

// Eval evaluates expr against row. Column references must have been bound by
// internal/validate; Resolved.Index selects the value from the row.
//
// Evaluation is deterministic and left-to-right. AND and OR short-circuit when
// the left operand decides the result, so an error in the right operand is not
// raised in that case.
func Eval(expr sqlparse.Expression, row catalog.Row) (catalog.Value, error) {
	if expr == nil {
		return catalog.Null(), &Error{Code: CodeExecutionError, Message: "Expression is nil."}
	}

	switch node := expr.(type) {
	case *sqlparse.Literal:
		return node.Value, nil

	case *sqlparse.ColumnRef:
		if node.Resolved == nil {
			return catalog.Null(), unboundColumn(node)
		}
		index := node.Resolved.Index
		if index < 0 || index >= len(row) {
			return catalog.Null(), errorAt(CodeExecutionError, node.Pos,
				"Row has no value for column %q at position %d.", node.Name, index)
		}
		return row[index], nil

	case *sqlparse.Unary:
		return evalUnary(node, row)

	case *sqlparse.Binary:
		return evalBinary(node, row)

	case *sqlparse.IsNull:
		value, err := Eval(node.Operand, row)
		if err != nil {
			return catalog.Null(), err
		}
		return catalog.Bool(value.IsNull() != node.Negated), nil

	case *sqlparse.Star:
		return catalog.Null(), errorAt(CodeTypeError, node.Pos,
			"A wildcard cannot be evaluated; expand it before evaluation.")

	default:
		return catalog.Null(), &Error{
			Code:    CodeExecutionError,
			Message: fmt.Sprintf("Unsupported expression node %T.", expr),
		}
	}
}

func evalUnary(node *sqlparse.Unary, row catalog.Row) (catalog.Value, error) {
	value, err := Eval(node.Operand, row)
	if err != nil {
		return catalog.Null(), err
	}

	switch node.Op {
	case sqlparse.OpNot:
		if value.IsNull() {
			return catalog.Null(), nil
		}
		boolean, ok := value.AsBool()
		if !ok {
			return catalog.Null(), errorAt(CodeTypeError, node.Pos,
				"NOT requires a boolean operand, found %s.", value.Type())
		}
		return catalog.Bool(!boolean), nil

	case sqlparse.OpNegate:
		if value.IsNull() {
			return catalog.Null(), nil
		}
		if integer, ok := value.AsInt(); ok {
			if integer == math.MinInt64 {
				return catalog.Null(), errorAt(CodeExecutionError, node.Pos,
					"Negating %d overflows a 64-bit integer.", integer)
			}
			return catalog.Int(-integer), nil
		}
		if decimal, ok := value.AsDecimal(); ok {
			return decimalResult(node.Pos, -decimal)
		}
		return catalog.Null(), errorAt(CodeTypeError, node.Pos,
			"Unary - requires a numeric operand, found %s.", value.Type())

	default:
		return catalog.Null(), &Error{
			Code:    CodeExecutionError,
			Message: fmt.Sprintf("Unsupported unary operator %d.", node.Op),
		}
	}
}

func evalBinary(node *sqlparse.Binary, row catalog.Row) (catalog.Value, error) {
	switch node.Op {
	case sqlparse.OpAnd:
		return evalAnd(node, row)
	case sqlparse.OpOr:
		return evalOr(node, row)
	}

	left, err := Eval(node.Left, row)
	if err != nil {
		return catalog.Null(), err
	}
	right, err := Eval(node.Right, row)
	if err != nil {
		return catalog.Null(), err
	}

	switch node.Op {
	case sqlparse.OpEqual, sqlparse.OpNotEqual,
		sqlparse.OpLess, sqlparse.OpLessEqual, sqlparse.OpGreater, sqlparse.OpGreaterEqual:
		return compare(node, left, right)

	case sqlparse.OpAdd, sqlparse.OpSubtract, sqlparse.OpMultiply, sqlparse.OpDivide:
		return arithmetic(node, left, right)

	default:
		return catalog.Null(), &Error{
			Code:    CodeExecutionError,
			Message: fmt.Sprintf("Unsupported binary operator %d.", node.Op),
		}
	}
}

func evalAnd(node *sqlparse.Binary, row catalog.Row) (catalog.Value, error) {
	left, err := Eval(node.Left, row)
	if err != nil {
		return catalog.Null(), err
	}
	leftBool, leftNull, err := booleanOperand(node.Left, left, "AND")
	if err != nil {
		return catalog.Null(), err
	}
	if !leftNull && !leftBool {
		return catalog.Bool(false), nil
	}

	right, err := Eval(node.Right, row)
	if err != nil {
		return catalog.Null(), err
	}
	rightBool, rightNull, err := booleanOperand(node.Right, right, "AND")
	if err != nil {
		return catalog.Null(), err
	}
	if !rightNull && !rightBool {
		return catalog.Bool(false), nil
	}

	if leftNull || rightNull {
		return catalog.Null(), nil
	}
	return catalog.Bool(true), nil
}

func evalOr(node *sqlparse.Binary, row catalog.Row) (catalog.Value, error) {
	left, err := Eval(node.Left, row)
	if err != nil {
		return catalog.Null(), err
	}
	leftBool, leftNull, err := booleanOperand(node.Left, left, "OR")
	if err != nil {
		return catalog.Null(), err
	}
	if !leftNull && leftBool {
		return catalog.Bool(true), nil
	}

	right, err := Eval(node.Right, row)
	if err != nil {
		return catalog.Null(), err
	}
	rightBool, rightNull, err := booleanOperand(node.Right, right, "OR")
	if err != nil {
		return catalog.Null(), err
	}
	if !rightNull && rightBool {
		return catalog.Bool(true), nil
	}

	if leftNull || rightNull {
		return catalog.Null(), nil
	}
	return catalog.Bool(false), nil
}

// booleanOperand unwraps a value used by AND or OR. The second result reports
// SQL NULL, which is neither true nor false.
func booleanOperand(expr sqlparse.Expression, value catalog.Value, operator string) (bool, bool, error) {
	if value.IsNull() {
		return false, true, nil
	}
	boolean, ok := value.AsBool()
	if !ok {
		return false, false, errorAt(CodeTypeError, expr.Position(),
			"%s requires boolean operands, found %s.", operator, value.Type())
	}
	return boolean, false, nil
}

func arithmetic(node *sqlparse.Binary, left, right catalog.Value) (catalog.Value, error) {
	leftInt, leftIsInt := left.AsInt()
	rightInt, rightIsInt := right.AsInt()
	_, leftIsDecimal := left.AsDecimal()
	_, rightIsDecimal := right.AsDecimal()

	if !left.IsNull() && !leftIsInt && !leftIsDecimal {
		return catalog.Null(), errorAt(CodeTypeError, node.Pos,
			"Operator %s requires numeric operands, found %s on the left.", binaryName(node.Op), left.Type())
	}
	if !right.IsNull() && !rightIsInt && !rightIsDecimal {
		return catalog.Null(), errorAt(CodeTypeError, node.Pos,
			"Operator %s requires numeric operands, found %s on the right.", binaryName(node.Op), right.Type())
	}
	if left.IsNull() || right.IsNull() {
		return catalog.Null(), nil
	}

	if leftIsInt && rightIsInt {
		return integerArithmetic(node, leftInt, rightInt)
	}
	return decimalArithmetic(node, numericAsFloat(left), numericAsFloat(right))
}

func integerArithmetic(node *sqlparse.Binary, left, right int64) (catalog.Value, error) {
	switch node.Op {
	case sqlparse.OpAdd:
		result := left + right
		if (right > 0 && result < left) || (right < 0 && result > left) {
			return catalog.Null(), overflow(node, "addition")
		}
		return catalog.Int(result), nil

	case sqlparse.OpSubtract:
		result := left - right
		if (right < 0 && result < left) || (right > 0 && result > left) {
			return catalog.Null(), overflow(node, "subtraction")
		}
		return catalog.Int(result), nil

	case sqlparse.OpMultiply:
		if left == 0 || right == 0 {
			return catalog.Int(0), nil
		}
		if (left == -1 && right == math.MinInt64) || (right == -1 && left == math.MinInt64) {
			return catalog.Null(), overflow(node, "multiplication")
		}
		result := left * right
		if result/left != right {
			return catalog.Null(), overflow(node, "multiplication")
		}
		return catalog.Int(result), nil

	case sqlparse.OpDivide:
		if right == 0 {
			return catalog.Null(), divisionByZero(node)
		}
		if left == math.MinInt64 && right == -1 {
			return catalog.Null(), overflow(node, "division")
		}
		return catalog.Int(left / right), nil

	default:
		return catalog.Null(), &Error{
			Code:    CodeExecutionError,
			Message: fmt.Sprintf("Unsupported arithmetic operator %d.", node.Op),
		}
	}
}

func decimalArithmetic(node *sqlparse.Binary, left, right float64) (catalog.Value, error) {
	var result float64

	switch node.Op {
	case sqlparse.OpAdd:
		result = left + right
	case sqlparse.OpSubtract:
		result = left - right
	case sqlparse.OpMultiply:
		result = left * right
	case sqlparse.OpDivide:
		if right == 0 {
			return catalog.Null(), divisionByZero(node)
		}
		result = left / right
	default:
		return catalog.Null(), &Error{
			Code:    CodeExecutionError,
			Message: fmt.Sprintf("Unsupported arithmetic operator %d.", node.Op),
		}
	}

	return decimalResult(node.Pos, result)
}

func compare(node *sqlparse.Binary, left, right catalog.Value) (catalog.Value, error) {
	if left.IsNull() || right.IsNull() {
		return catalog.Null(), nil
	}

	leftInt, leftIsInt := left.AsInt()
	rightInt, rightIsInt := right.AsInt()
	_, leftIsDecimal := left.AsDecimal()
	_, rightIsDecimal := right.AsDecimal()

	if (leftIsInt || leftIsDecimal) && (rightIsInt || rightIsDecimal) {
		if leftIsInt && rightIsInt {
			return ordered(node.Op, leftInt, rightInt)
		}
		return ordered(node.Op, numericAsFloat(left), numericAsFloat(right))
	}

	leftText, leftIsText := left.AsText()
	rightText, rightIsText := right.AsText()
	if leftIsText && rightIsText {
		return ordered(node.Op, leftText, rightText)
	}

	leftBool, leftIsBool := left.AsBool()
	rightBool, rightIsBool := right.AsBool()
	if leftIsBool && rightIsBool {
		if node.Op != sqlparse.OpEqual && node.Op != sqlparse.OpNotEqual {
			return catalog.Null(), errorAt(CodeTypeError, node.Pos,
				"Booleans only support = and !=; %s has no boolean ordering.", comparisonName(node.Op))
		}
		return catalog.Bool((leftBool == rightBool) == (node.Op == sqlparse.OpEqual)), nil
	}

	return catalog.Null(), errorAt(CodeTypeError, node.Pos,
		"Cannot compare %s with %s.", left.Type(), right.Type())
}

// Compare orders two non-NULL values of compatible types. It returns -1 when
// left sorts before right, 0 when they are equal, and 1 otherwise. NULL handling
// belongs to the caller, because where NULLs sort is a query-level decision.
func Compare(left, right catalog.Value) (int, error) {
	if left.IsNull() || right.IsNull() {
		return 0, &Error{Code: CodeExecutionError, Message: "Compare requires non-NULL values."}
	}

	leftInt, leftIsInt := left.AsInt()
	rightInt, rightIsInt := right.AsInt()
	_, leftIsDecimal := left.AsDecimal()
	_, rightIsDecimal := right.AsDecimal()

	if (leftIsInt || leftIsDecimal) && (rightIsInt || rightIsDecimal) {
		if leftIsInt && rightIsInt {
			return cmp.Compare(leftInt, rightInt), nil
		}
		return cmp.Compare(numericAsFloat(left), numericAsFloat(right)), nil
	}

	leftText, leftIsText := left.AsText()
	rightText, rightIsText := right.AsText()
	if leftIsText && rightIsText {
		return strings.Compare(leftText, rightText), nil
	}

	if left.Type() == catalog.TypeBoolean && right.Type() == catalog.TypeBoolean {
		return 0, &Error{Code: CodeTypeError, Message: "Booleans have no ordering; use = or !=."}
	}

	return 0, &Error{Code: CodeTypeError,
		Message: fmt.Sprintf("Cannot compare %s with %s.", left.Type(), right.Type())}
}

// ordered applies a comparison to two values of the same ordered type.
func ordered[T cmp.Ordered](op sqlparse.BinaryOp, left, right T) (catalog.Value, error) {
	switch op {
	case sqlparse.OpEqual:
		return catalog.Bool(left == right), nil
	case sqlparse.OpNotEqual:
		return catalog.Bool(left != right), nil
	case sqlparse.OpLess:
		return catalog.Bool(left < right), nil
	case sqlparse.OpLessEqual:
		return catalog.Bool(left <= right), nil
	case sqlparse.OpGreater:
		return catalog.Bool(left > right), nil
	case sqlparse.OpGreaterEqual:
		return catalog.Bool(left >= right), nil
	default:
		return catalog.Null(), &Error{
			Code:    CodeExecutionError,
			Message: fmt.Sprintf("Unsupported comparison operator %d.", op),
		}
	}
}

func numericAsFloat(value catalog.Value) float64 {
	if integer, ok := value.AsInt(); ok {
		return float64(integer)
	}
	decimal, _ := value.AsDecimal()
	return decimal
}

func decimalResult(pos sqlparse.Position, value float64) (catalog.Value, error) {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return catalog.Null(), errorAt(CodeExecutionError, pos, "Decimal result is out of range.")
	}
	return catalog.Decimal(value), nil
}

func divisionByZero(node *sqlparse.Binary) *Error {
	return errorAt(CodeExecutionError, node.Pos, "Division by zero.")
}

func overflow(node *sqlparse.Binary, operation string) *Error {
	return errorAt(CodeExecutionError, node.Pos,
		"Integer %s overflows a 64-bit integer.", operation)
}
