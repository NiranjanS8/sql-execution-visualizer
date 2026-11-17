package query

import (
	"errors"
	"fmt"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/eval"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

// Code classifies a query failure. The values match the stable codes documented
// in docs/api.md, so the API can pass them through without a translation table.
type Code string

const (
	CodeInvalidRequest    Code = "invalid_request"
	CodeParseError        Code = "parse_error"
	CodeUnsupportedSyntax Code = "unsupported_syntax"
	CodeUnknownTable      Code = "unknown_table"
	CodeUnknownColumn     Code = "unknown_column"
	CodeAmbiguousColumn   Code = "ambiguous_column"
	CodeTypeError         Code = "type_error"
	CodeExecutionError    Code = "execution_error"
	CodeTimeout           Code = "timeout"
	CodeCancelled         Code = "cancelled"
	CodeResultLimit       Code = "result_limit_exceeded"
)

// Error is a user-facing query failure. Line and Column are 1-based and zero
// when the failure has no source position. cause is set only for unexpected
// execution failures; it is logged by the API and never sent to the browser.
type Error struct {
	Code    Code
	Message string
	Line    int
	Column  int
	cause   error
}

func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Message)
	}
	return e.Message
}

// Unwrap exposes the underlying failure, if any, so the API can log it.
func (e *Error) Unwrap() error { return e.cause }

// classifyPipelineError converts the known parse and evaluation error types into
// a query error. Any other failure is a programming error and is wrapped so the
// API reports it as an internal failure rather than blaming the user's SQL.
func classifyPipelineError(err error) error {
	if converted, ok := convertKnown(err); ok {
		return converted
	}
	return fmt.Errorf("query: unexpected pipeline error: %w", err)
}

func convertKnown(err error) (*Error, bool) {
	var parseErr *sqlparse.Error
	if errors.As(err, &parseErr) {
		return &Error{
			Code:    Code(parseErr.Code),
			Message: parseErr.Message,
			Line:    parseErr.Line,
			Column:  parseErr.Column,
		}, true
	}

	var evalErr *eval.Error
	if errors.As(err, &evalErr) {
		return &Error{
			Code:    Code(evalErr.Code),
			Message: evalErr.Message,
			Line:    evalErr.Line,
			Column:  evalErr.Column,
		}, true
	}

	return nil, false
}
