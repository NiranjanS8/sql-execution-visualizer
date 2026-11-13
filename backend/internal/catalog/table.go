package catalog

import (
	"fmt"
	"strings"
	"time"
)

// TableID is a stable table identifier. It is independent of the display name so a
// table can be renamed without breaking references to it.
type TableID string

// Row is one record, positionally aligned with its table's schema.
type Row []Value

// Source records where a table came from. It is nil for built-in tables.
type Source struct {
	Filename   string
	ImportedAt time.Time
}

// Table is a named, ordered set of rows. A table is immutable once registered in a
// Catalog: no catalog operation mutates its schema or rows after creation.
type Table struct {
	ID     TableID
	Name   string
	Schema Schema
	Rows   []Row
	Source *Source
}

// Validate reports the first definition problem in the table.
func (t *Table) Validate() error {
	if t == nil {
		return invalid("", "table is nil")
	}
	if strings.TrimSpace(string(t.ID)) == "" {
		return invalid("id", "table ID must not be empty")
	}
	if !identifierPattern.MatchString(t.Name) {
		return invalid("name",
			"%q is not a valid table name; expected letters, digits, and underscores, starting with a letter or underscore",
			t.Name)
	}
	if t.Schema.Len() == 0 {
		return invalid("schema", "table must declare a schema")
	}
	if err := t.validateColumnNames(); err != nil {
		return err
	}

	for position, row := range t.Rows {
		if err := t.validateRow(position, row); err != nil {
			return err
		}
	}

	return nil
}

func (t *Table) validateColumnNames() error {
	seen := make(map[string]struct{}, t.Schema.Len())
	for _, column := range t.Schema.Columns() {
		key := strings.ToLower(column.Name)
		if _, exists := seen[key]; exists {
			return invalid("columns", "duplicate column name %q", column.Name)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (t *Table) validateRow(position int, row Row) error {
	if len(row) != t.Schema.Len() {
		return invalid(fmt.Sprintf("rows[%d]", position),
			"expected %d values, found %d", t.Schema.Len(), len(row))
	}

	for i, value := range row {
		column := t.Schema.ColumnAt(i)
		field := fmt.Sprintf("rows[%d].%s", position, column.Name)

		if value.IsNull() {
			if column.Nullability == NullabilityNotNull {
				return invalid(field, "column does not allow NULL")
			}
			continue
		}
		if value.Type() != column.Type {
			return invalid(field, "expected %s, found %s", column.Type, value.Type())
		}
	}

	return nil
}

// RowCount returns the number of rows.
func (t *Table) RowCount() int { return len(t.Rows) }

// ColumnCount returns the number of columns.
func (t *Table) ColumnCount() int { return t.Schema.Len() }
