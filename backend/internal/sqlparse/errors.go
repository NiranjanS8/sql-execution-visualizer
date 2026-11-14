package sqlparse

import "fmt"

// ErrorCode classifies a SQL failure. The values are stable: the API maps them
// to HTTP responses and the frontend matches on them.
type ErrorCode string

const (
	// CodeParseError means the text is not valid SQL in the supported grammar.
	CodeParseError ErrorCode = "parse_error"
	// CodeUnsupportedSyntax means the text is valid SQL but outside the subset.
	CodeUnsupportedSyntax ErrorCode = "unsupported_syntax"
	// CodeUnknownTable means the FROM table is not in the catalog.
	CodeUnknownTable ErrorCode = "unknown_table"
	// CodeUnknownColumn means a column reference does not resolve.
	CodeUnknownColumn ErrorCode = "unknown_column"
	// CodeAmbiguousColumn means a column reference matches more than one source.
	CodeAmbiguousColumn ErrorCode = "ambiguous_column"
)

// Error is a SQL syntax or resolution failure. Line and Column are 1-based and
// zero when the failure has no single source position.
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

// errorAt builds an Error at a token position.
func errorAt(code ErrorCode, line, column int, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...), Line: line, Column: column}
}
