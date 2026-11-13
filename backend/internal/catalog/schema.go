package catalog

import (
	"fmt"
	"regexp"
	"strconv"
)

// ColumnID identifies a column within a schema. It stays stable when a schema is
// reordered and remains unique when a join introduces duplicate column names.
type ColumnID string

// Nullability records what is known about NULLs in a column.
type Nullability string

const (
	NullabilityUnknown  Nullability = "unknown"
	NullabilityNullable Nullability = "nullable"
	NullabilityNotNull  Nullability = "not_null"
)

// Column is one ordered field of a schema.
type Column struct {
	ID          ColumnID
	Name        string
	Type        Type
	Nullability Nullability
}

// ColumnDef declares a column before the schema assigns it an identifier.
type ColumnDef struct {
	Name        string
	Type        Type
	Nullability Nullability
}

// Schema is an ordered set of columns. Duplicate names are permitted because a
// join can produce them; access by identifier stays unambiguous.
type Schema struct {
	columns []Column
	index   map[ColumnID]int
}

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// NewSchema assigns identifiers c1..cN in declaration order and validates the
// result.
func NewSchema(defs ...ColumnDef) (Schema, error) {
	if len(defs) == 0 {
		return Schema{}, invalid("columns", "schema must declare at least one column")
	}

	schema := Schema{
		columns: make([]Column, len(defs)),
		index:   make(map[ColumnID]int, len(defs)),
	}

	for position, def := range defs {
		column := Column{
			ID:          ColumnID("c" + strconv.Itoa(position+1)),
			Name:        def.Name,
			Type:        def.Type,
			Nullability: def.Nullability,
		}
		if column.Nullability == "" {
			column.Nullability = NullabilityUnknown
		}
		if err := validateColumn(column, position); err != nil {
			return Schema{}, err
		}
		schema.columns[position] = column
		schema.index[column.ID] = position
	}

	return schema, nil
}

func validateColumn(column Column, position int) error {
	field := fmt.Sprintf("columns[%d]", position)

	if !identifierPattern.MatchString(column.Name) {
		return invalid(field+".name",
			"%q is not a valid column name; expected letters, digits, and underscores, starting with a letter or underscore",
			column.Name)
	}
	if !column.Type.IsColumnType() {
		return invalid(field+".type", "%q is not a column type", column.Type)
	}
	switch column.Nullability {
	case NullabilityUnknown, NullabilityNullable, NullabilityNotNull:
	default:
		return invalid(field+".nullability", "%q is not a known nullability", column.Nullability)
	}

	return nil
}

// Len returns the number of columns.
func (s Schema) Len() int { return len(s.columns) }

// Columns returns the ordered columns. The result is a copy.
func (s Schema) Columns() []Column {
	return append([]Column(nil), s.columns...)
}

// ColumnAt returns the column at position i.
func (s Schema) ColumnAt(i int) Column { return s.columns[i] }

// Column looks a column up by identifier.
func (s Schema) Column(id ColumnID) (Column, bool) {
	position, ok := s.index[id]
	if !ok {
		return Column{}, false
	}
	return s.columns[position], true
}

// Index returns the position of the column with the given identifier.
func (s Schema) Index(id ColumnID) (int, bool) {
	position, ok := s.index[id]
	return position, ok
}

// Names returns the column names in schema order.
func (s Schema) Names() []string {
	names := make([]string, len(s.columns))
	for i, column := range s.columns {
		names[i] = column.Name
	}
	return names
}
