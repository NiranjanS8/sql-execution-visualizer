// Package eval infers the type of a parsed expression and evaluates it against a
// catalog row.
//
// SQL NULL is not a sentinel for false or zero: comparisons and arithmetic with
// NULL produce NULL, NOT NULL is NULL, and AND/OR follow the SQL three-valued
// truth tables. IS NULL and IS NOT NULL are the only operators that always return
// TRUE or FALSE.
//
// The package does not parse SQL and does not resolve names; it expects column
// references that internal/validate has already bound.
package eval

import (
	"fmt"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

// ErrorCode classifies an evaluation failure. The values match the codes reserved
// in docs/api.md so the query layer can pass them through unchanged.
type ErrorCode string

const (
	// CodeTypeError means the operands are not valid for the operator.
	CodeTypeError ErrorCode = "type_error"
	// CodeExecutionError means evaluation failed for a reason other than types,
	// such as division by zero, numeric overflow, or an unbound column.
	CodeExecutionError ErrorCode = "execution_error"
)

// Error is an expression type or evaluation failure. Line and Column are 1-based
// and zero when the failure has no source position.
type Error struct {
	Code    ErrorCode
	Message string
	Line    int
	Column  int
}

func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Message)
	}
	return e.Message
}

func errorAt(code ErrorCode, pos sqlparse.Position, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...), Line: pos.Line, Column: pos.Column}
}
