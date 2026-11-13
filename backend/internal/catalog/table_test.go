package catalog

import (
	"strings"
	"testing"
)

func tableWithColumns(t *testing.T, rows []Row) *Table {
	t.Helper()

	schema, err := NewSchema(
		ColumnDef{Name: "id", Type: TypeInteger, Nullability: NullabilityNotNull},
		ColumnDef{Name: "name", Type: TypeText, Nullability: NullabilityNotNull},
		ColumnDef{Name: "salary", Type: TypeInteger, Nullability: NullabilityNullable},
	)
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}

	return &Table{ID: "people", Name: "people", Schema: schema, Rows: rows}
}

func TestTableValidateAcceptsWellFormedRows(t *testing.T) {
	table := tableWithColumns(t, []Row{
		{Int(1), Text("Ada"), Int(145_000)},
		{Int(2), Text("Grace"), Null()},
	})

	if err := table.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := table.RowCount(); got != 2 {
		t.Errorf("RowCount() = %d, want 2", got)
	}
	if got := table.ColumnCount(); got != 3 {
		t.Errorf("ColumnCount() = %d, want 3", got)
	}
}

func TestTableValidateRejectsRowWidthMismatch(t *testing.T) {
	table := tableWithColumns(t, []Row{{Int(1), Text("Ada")}})

	err := table.Validate()
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if !strings.Contains(err.Error(), "expected 3 values, found 2") {
		t.Errorf("error = %q, want a row-width message", err)
	}
}

func TestTableValidateRejectsNullInNotNullColumn(t *testing.T) {
	table := tableWithColumns(t, []Row{{Null(), Text("Ada"), Int(1)}})

	err := table.Validate()
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if !strings.Contains(err.Error(), "rows[0].id") {
		t.Errorf("error = %q, want the offending field path", err)
	}
	if !strings.Contains(err.Error(), "does not allow NULL") {
		t.Errorf("error = %q, want a nullability message", err)
	}
}

func TestTableValidateRejectsMismatchedValueType(t *testing.T) {
	table := tableWithColumns(t, []Row{{Int(1), Text("Ada"), Text("145000")}})

	err := table.Validate()
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if !strings.Contains(err.Error(), "rows[0].salary") {
		t.Errorf("error = %q, want the offending field path", err)
	}
	if !strings.Contains(err.Error(), "expected integer, found text") {
		t.Errorf("error = %q, want a type message", err)
	}
}

func TestTableValidateRejectsInvalidIdentity(t *testing.T) {
	tests := []struct {
		name      string
		id        TableID
		tableName string
	}{
		{"empty id", "", "people"},
		{"blank id", "   ", "people"},
		{"empty name", "people", ""},
		{"name with a space", "people", "my table"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			table := tableWithColumns(t, nil)
			table.ID = test.id
			table.Name = test.tableName

			if err := table.Validate(); err == nil {
				t.Fatal("expected a validation error")
			}
		})
	}
}

func TestTableValidateRejectsDuplicateColumnNames(t *testing.T) {
	schema, err := NewSchema(
		ColumnDef{Name: "id", Type: TypeInteger},
		ColumnDef{Name: "ID", Type: TypeInteger},
	)
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}

	table := &Table{ID: "people", Name: "people", Schema: schema, Rows: []Row{{Int(1), Int(2)}}}

	err = table.Validate()
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if !strings.Contains(err.Error(), "duplicate column name") {
		t.Errorf("error = %q, want a duplicate-column message", err)
	}
}

func TestTableValidateRejectsNilTable(t *testing.T) {
	var table *Table

	if err := table.Validate(); err == nil {
		t.Fatal("expected an error for a nil table")
	}
}
