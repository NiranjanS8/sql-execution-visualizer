package catalog

import (
	"errors"
	"fmt"
)

var (
	// ErrTableExists is returned when a table ID or name is already registered.
	ErrTableExists = errors.New("table already exists")
	// ErrTableNotFound is returned when a table ID is not registered.
	ErrTableNotFound = errors.New("table not found")
)

// ValidationError reports a definition problem at a specific field path, such as
// "rows[3].salary". Field is empty when the problem is not attributable to one
// field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

func invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}
