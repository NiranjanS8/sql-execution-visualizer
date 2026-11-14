package eval

import (
	"fmt"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

// Infer returns the static type of expr without evaluating it. It reports
// type_error when an operator cannot accept its operands, and execution_error
// when the expression was not bound by internal/validate.
//
// TypeNull means "unknown", not "a column type": it is compatible with any
// operand, and the operation itself yields NULL.
func Infer(expr sqlparse.Expression) (catalog.Type, error) {
	if expr == nil {
		return catalog.TypeNull, &Error{Code: CodeExecutionError, Message: "Expression is nil."}
	}

	switch node := expr.(type) {
	case *sqlparse.Literal:
		return node.Value.Type(), nil

	case *sqlparse.ColumnRef:
		if node.Resolved == nil {
			return catalog.TypeNull, unboundColumn(node)
		}
		return node.Resolved.Column.Type, nil

	case *sqlparse.Unary:
		return inferUnary(node)

	case *sqlparse.Binary:
		return inferBinary(node)

	case *sqlparse.IsNull:
		if _, err := Infer(node.Operand); err != nil {
			return catalog.TypeNull, err
		}
		return catalog.TypeBoolean, nil

	case *sqlparse.Star:
		return catalog.TypeNull, errorAt(CodeTypeError, node.Pos,
			"A wildcard cannot be evaluated; expand it before type checking.")

	default:
		return catalog.TypeNull, &Error{
			Code:    CodeExecutionError,
			Message: fmt.Sprintf("Unsupported expression node %T.", expr),
		}
	}
}

func inferUnary(node *sqlparse.Unary) (catalog.Type, error) {
	operand, err := Infer(node.Operand)
	if err != nil {
		return catalog.TypeNull, err
	}

	switch node.Op {
	case sqlparse.OpNot:
		if !acceptsBoolean(operand) {
			return catalog.TypeNull, errorAt(CodeTypeError, node.Pos,
				"NOT requires a boolean operand, found %s.", operand)
		}
		return catalog.TypeBoolean, nil

	case sqlparse.OpNegate:
		if !acceptsNumeric(operand) {
			return catalog.TypeNull, errorAt(CodeTypeError, node.Pos,
				"Unary - requires a numeric operand, found %s.", operand)
		}
		return operand, nil

	default:
		return catalog.TypeNull, &Error{
			Code:    CodeExecutionError,
			Message: fmt.Sprintf("Unsupported unary operator %d.", node.Op),
		}
	}
}

func inferBinary(node *sqlparse.Binary) (catalog.Type, error) {
	left, err := Infer(node.Left)
	if err != nil {
		return catalog.TypeNull, err
	}
	right, err := Infer(node.Right)
	if err != nil {
		return catalog.TypeNull, err
	}

	switch node.Op {
	case sqlparse.OpAnd, sqlparse.OpOr:
		if !acceptsBoolean(left) {
			return catalog.TypeNull, errorAt(CodeTypeError, node.Pos,
				"%s requires boolean operands, found %s on the left.", logicalName(node.Op), left)
		}
		if !acceptsBoolean(right) {
			return catalog.TypeNull, errorAt(CodeTypeError, node.Pos,
				"%s requires boolean operands, found %s on the right.", logicalName(node.Op), right)
		}
		return catalog.TypeBoolean, nil

	case sqlparse.OpEqual, sqlparse.OpNotEqual,
		sqlparse.OpLess, sqlparse.OpLessEqual, sqlparse.OpGreater, sqlparse.OpGreaterEqual:
		if err := checkComparison(node.Op, left, right, node.Pos); err != nil {
			return catalog.TypeNull, err
		}
		return catalog.TypeBoolean, nil

	case sqlparse.OpAdd, sqlparse.OpSubtract, sqlparse.OpMultiply, sqlparse.OpDivide:
		return numericResult(node.Op, left, right, node.Pos)

	default:
		return catalog.TypeNull, &Error{
			Code:    CodeExecutionError,
			Message: fmt.Sprintf("Unsupported binary operator %d.", node.Op),
		}
	}
}

func checkComparison(op sqlparse.BinaryOp, left, right catalog.Type, pos sqlparse.Position) error {
	if left == catalog.TypeNull || right == catalog.TypeNull {
		return nil
	}
	if isNumeric(left) && isNumeric(right) {
		return nil
	}
	if left != right {
		return errorAt(CodeTypeError, pos, "Cannot compare %s with %s.", left, right)
	}
	if left == catalog.TypeBoolean && op != sqlparse.OpEqual && op != sqlparse.OpNotEqual {
		return errorAt(CodeTypeError, pos,
			"Booleans only support = and !=; %s has no boolean ordering.", comparisonName(op))
	}
	return nil
}

func numericResult(op sqlparse.BinaryOp, left, right catalog.Type, pos sqlparse.Position) (catalog.Type, error) {
	if !acceptsNumeric(left) {
		return catalog.TypeNull, errorAt(CodeTypeError, pos,
			"Operator %s requires numeric operands, found %s on the left.", binaryName(op), left)
	}
	if !acceptsNumeric(right) {
		return catalog.TypeNull, errorAt(CodeTypeError, pos,
			"Operator %s requires numeric operands, found %s on the right.", binaryName(op), right)
	}

	switch {
	case left == catalog.TypeNull:
		return right, nil
	case right == catalog.TypeNull:
		return left, nil
	case left == catalog.TypeInteger && right == catalog.TypeInteger:
		return catalog.TypeInteger, nil
	default:
		return catalog.TypeDecimal, nil
	}
}

func acceptsBoolean(t catalog.Type) bool {
	return t == catalog.TypeBoolean || t == catalog.TypeNull
}

func acceptsNumeric(t catalog.Type) bool {
	return t == catalog.TypeInteger || t == catalog.TypeDecimal || t == catalog.TypeNull
}

func isNumeric(t catalog.Type) bool {
	return t == catalog.TypeInteger || t == catalog.TypeDecimal
}

func unboundColumn(ref *sqlparse.ColumnRef) *Error {
	return errorAt(CodeExecutionError, ref.Pos,
		"Column %q is not bound; validate the query before evaluating it.", ref.Name)
}

func logicalName(op sqlparse.BinaryOp) string {
	if op == sqlparse.OpOr {
		return "OR"
	}
	return "AND"
}

func binaryName(op sqlparse.BinaryOp) string {
	switch op {
	case sqlparse.OpAdd:
		return "+"
	case sqlparse.OpSubtract:
		return "-"
	case sqlparse.OpMultiply:
		return "*"
	case sqlparse.OpDivide:
		return "/"
	default:
		return "?"
	}
}

func comparisonName(op sqlparse.BinaryOp) string {
	switch op {
	case sqlparse.OpEqual:
		return "="
	case sqlparse.OpNotEqual:
		return "!="
	case sqlparse.OpLess:
		return "<"
	case sqlparse.OpLessEqual:
		return "<="
	case sqlparse.OpGreater:
		return ">"
	case sqlparse.OpGreaterEqual:
		return ">="
	default:
		return "?"
	}
}
