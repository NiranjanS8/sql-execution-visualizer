package planner

import (
	"fmt"
	"strings"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

// formatExpr renders an expression for explain output. It is a debug format, not
// a SQL round-trip: binary expressions are fully parenthesized so precedence can
// never be misread, and literals use catalog.Value's SQL spelling.
func formatExpr(expr sqlparse.Expression) string {
	switch node := expr.(type) {
	case *sqlparse.Literal:
		return node.Value.String()

	case *sqlparse.ColumnRef:
		if node.Table != "" {
			return node.Table + "." + node.Name
		}
		return node.Name

	case *sqlparse.Star:
		if node.Table != "" {
			return node.Table + ".*"
		}
		return "*"

	case *sqlparse.Unary:
		if node.Op == sqlparse.OpNot {
			return "NOT " + formatExpr(node.Operand)
		}
		return "-" + formatExpr(node.Operand)

	case *sqlparse.Binary:
		return "(" + formatExpr(node.Left) + " " + binarySymbol(node.Op) + " " + formatExpr(node.Right) + ")"

	case *sqlparse.IsNull:
		if node.Negated {
			return formatExpr(node.Operand) + " IS NOT NULL"
		}
		return formatExpr(node.Operand) + " IS NULL"

	default:
		return fmt.Sprintf("<%T>", expr)
	}
}

func binarySymbol(op sqlparse.BinaryOp) string {
	switch op {
	case sqlparse.OpOr:
		return "OR"
	case sqlparse.OpAnd:
		return "AND"
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

// Explain renders the plan as an indented tree, one node per line. It is stable
// enough to assert in tests and small enough to read in a debugger.
func (p *Plan) Explain() string {
	if p == nil || p.Root == nil {
		return ""
	}

	var out []string
	appendNode(&out, p.Root, 0)
	return strings.Join(out, "\n")
}

func appendNode(out *[]string, node Node, depth int) {
	line := strings.Repeat("  ", depth) + string(node.Kind())
	if detail := node.Describe(); detail != "" {
		line += " " + detail
	}
	*out = append(*out, line)

	for _, child := range node.Children() {
		appendNode(out, child, depth+1)
	}
}
