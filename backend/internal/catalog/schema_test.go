package catalog

import (
	"errors"
	"reflect"
	"testing"
)

func TestNewSchemaAssignsStableIDsInOrder(t *testing.T) {
	schema, err := NewSchema(
		ColumnDef{Name: "id", Type: TypeInteger, Nullability: NullabilityNotNull},
		ColumnDef{Name: "name", Type: TypeText, Nullability: NullabilityNotNull},
		ColumnDef{Name: "salary", Type: TypeInteger, Nullability: NullabilityNullable},
	)
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}

	if got := schema.Len(); got != 3 {
		t.Fatalf("Len() = %d, want 3", got)
	}

	for position, want := range []ColumnID{"c1", "c2", "c3"} {
		if got := schema.ColumnAt(position).ID; got != want {
			t.Errorf("ColumnAt(%d).ID = %q, want %q", position, got, want)
		}
	}

	wantNames := []string{"id", "name", "salary"}
	if got := schema.Names(); !reflect.DeepEqual(got, wantNames) {
		t.Errorf("Names() = %v, want %v", got, wantNames)
	}
}

func TestSchemaLookupByID(t *testing.T) {
	schema, err := NewSchema(ColumnDef{Name: "id", Type: TypeInteger})
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}

	column, ok := schema.Column("c1")
	if !ok {
		t.Fatal("c1 was not found")
	}
	if column.Name != "id" {
		t.Errorf("c1 name = %q, want id", column.Name)
	}

	if _, ok := schema.Column("c9"); ok {
		t.Error("an unknown identifier resolved")
	}

	position, ok := schema.Index("c1")
	if !ok || position != 0 {
		t.Errorf("Index(c1) = %d, %v; want 0, true", position, ok)
	}
}

func TestNewSchemaNormalizesUnknownNullability(t *testing.T) {
	schema, err := NewSchema(ColumnDef{Name: "note", Type: TypeText})
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}

	if got := schema.ColumnAt(0).Nullability; got != NullabilityUnknown {
		t.Errorf("nullability = %q, want %q", got, NullabilityUnknown)
	}
}

func TestSchemaColumnsReturnsCopy(t *testing.T) {
	schema, err := NewSchema(ColumnDef{Name: "id", Type: TypeInteger})
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}

	columns := schema.Columns()
	columns[0].Name = "mutated"

	if got := schema.ColumnAt(0).Name; got != "id" {
		t.Errorf("mutating the returned columns changed the schema: %q", got)
	}
}

func TestNewSchemaRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name string
		defs []ColumnDef
	}{
		{"no columns", nil},
		{"empty name", []ColumnDef{{Name: "", Type: TypeText}}},
		{"name with a space", []ColumnDef{{Name: "full name", Type: TypeText}}},
		{"name starting with a digit", []ColumnDef{{Name: "1st", Type: TypeText}}},
		{"name with a dash", []ColumnDef{{Name: "first-name", Type: TypeText}}},
		{"null column type", []ColumnDef{{Name: "ghost", Type: TypeNull}}},
		{"unknown type", []ColumnDef{{Name: "mystery", Type: "geometry"}}},
		{"unknown nullability", []ColumnDef{{Name: "note", Type: TypeText, Nullability: "sometimes"}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewSchema(test.defs...)
			if err == nil {
				t.Fatal("expected a validation error")
			}

			var validation *ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("error type = %T, want *ValidationError", err)
			}
			if validation.Field == "" {
				t.Error("ValidationError.Field is empty")
			}
		})
	}
}

func TestSchemaAllowsDuplicateColumnNames(t *testing.T) {
	schema, err := NewSchema(
		ColumnDef{Name: "id", Type: TypeInteger},
		ColumnDef{Name: "id", Type: TypeInteger},
	)
	if err != nil {
		t.Fatalf("duplicate names must be allowed for join output: %v", err)
	}

	if schema.ColumnAt(0).ID == schema.ColumnAt(1).ID {
		t.Error("duplicate columns share an identifier")
	}
}
