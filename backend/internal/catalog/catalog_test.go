package catalog

import (
	"errors"
	"testing"
)

func simpleTable(t *testing.T, id TableID, name string) *Table {
	t.Helper()

	schema, err := NewSchema(ColumnDef{Name: "id", Type: TypeInteger, Nullability: NullabilityNotNull})
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}

	return &Table{ID: id, Name: name, Schema: schema, Rows: []Row{{Int(1)}}}
}

func TestCatalogCreateAndGet(t *testing.T) {
	store := New()
	table := simpleTable(t, "people", "people")

	if err := store.Create(table); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := store.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}

	found, ok := store.Get("people")
	if !ok {
		t.Fatal("Get did not find the registered table")
	}
	if found != table {
		t.Error("Get returned a different table instance")
	}
}

func TestCatalogGetUnknownTable(t *testing.T) {
	if _, ok := New().Get("missing"); ok {
		t.Error("Get found a table that was never registered")
	}
}

func TestCatalogListPreservesRegistrationOrder(t *testing.T) {
	store := New()
	ids := []TableID{"zebra", "alpha", "middle"}

	for _, id := range ids {
		if err := store.Create(simpleTable(t, id, string(id))); err != nil {
			t.Fatalf("Create(%s): %v", id, err)
		}
	}

	listed := store.List()
	if len(listed) != len(ids) {
		t.Fatalf("List() returned %d tables, want %d", len(listed), len(ids))
	}
	for i, id := range ids {
		if listed[i].ID != id {
			t.Errorf("List()[%d].ID = %q, want %q", i, listed[i].ID, id)
		}
	}
}

func TestCatalogListReturnsIndependentSlice(t *testing.T) {
	store := New()
	if err := store.Create(simpleTable(t, "people", "people")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	listed := store.List()
	listed[0] = nil

	if got := store.List()[0]; got == nil {
		t.Error("mutating the returned slice changed the catalog")
	}
}

func TestCatalogRejectsDuplicateID(t *testing.T) {
	store := New()
	if err := store.Create(simpleTable(t, "people", "people")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := store.Create(simpleTable(t, "people", "other"))
	if !errors.Is(err, ErrTableExists) {
		t.Fatalf("error = %v, want ErrTableExists", err)
	}
	if got := store.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}
}

func TestCatalogRejectsDuplicateNameIgnoringCase(t *testing.T) {
	store := New()
	if err := store.Create(simpleTable(t, "people", "people")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	err := store.Create(simpleTable(t, "people_copy", "PEOPLE"))
	if !errors.Is(err, ErrTableExists) {
		t.Fatalf("error = %v, want ErrTableExists", err)
	}
	if got := store.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}
}

func TestCatalogRejectsInvalidTable(t *testing.T) {
	store := New()
	broken := simpleTable(t, "people", "people")
	broken.Rows = []Row{{Int(1), Int(2)}}

	err := store.Create(broken)

	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v, want a *ValidationError", err)
	}
	if got := store.Len(); got != 0 {
		t.Errorf("an invalid table was registered (Len() = %d)", got)
	}
}

func TestCatalogRejectsNilTable(t *testing.T) {
	var table *Table

	if err := New().Create(table); err == nil {
		t.Fatal("expected an error for a nil table")
	}
}

func TestCatalogDelete(t *testing.T) {
	store := New()
	if err := store.Create(simpleTable(t, "people", "people")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Create(simpleTable(t, "orders", "orders")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Delete("people"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := store.Get("people"); ok {
		t.Error("a deleted table is still registered")
	}

	listed := store.List()
	if len(listed) != 1 || listed[0].ID != "orders" {
		t.Errorf("List() = %v, want only orders", listed)
	}

	if err := store.Create(simpleTable(t, "people", "people")); err != nil {
		t.Errorf("the deleted name was not released for reuse: %v", err)
	}
}

func TestCatalogDeleteUnknown(t *testing.T) {
	err := New().Delete("missing")

	if !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("error = %v, want ErrTableNotFound", err)
	}
}
