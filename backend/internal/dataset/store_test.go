package dataset

import (
	"errors"
	"testing"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
)

func testTable(t *testing.T, id catalog.TableID, name string) *catalog.Table {
	t.Helper()

	schema, err := catalog.NewSchema(catalog.ColumnDef{Name: "id", Type: catalog.TypeInteger})
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}

	return &catalog.Table{ID: id, Name: name, Schema: schema, Rows: []catalog.Row{{catalog.Int(1)}}}
}

func TestStoreCreateGetAndList(t *testing.T) {
	store := NewStore()

	created, err := store.Create("Employees", Source{Filename: "employees.csv", Bytes: 42}, testTable(t, "employees", "employees"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != "employees" {
		t.Errorf("ID = %q, want employees", created.ID)
	}
	if created.Name != "Employees" {
		t.Errorf("Name = %q, want Employees", created.Name)
	}
	if created.RowCount() != 1 {
		t.Errorf("RowCount() = %d, want 1", created.RowCount())
	}

	found, ok := store.Get("employees")
	if !ok {
		t.Fatal("Get did not find the dataset")
	}
	if found != created {
		t.Error("Get returned a different dataset instance")
	}

	if got := store.Len(); got != 1 {
		t.Errorf("Len() = %d, want 1", got)
	}
	listed := store.List()
	if len(listed) != 1 || listed[0].ID != "employees" {
		t.Errorf("List() = %v, want only employees", listed)
	}
}

func TestStoreGeneratesUniqueIDs(t *testing.T) {
	store := NewStore()

	first, err := store.Create("Sales", Source{}, testTable(t, "sales", "sales"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	second, err := store.Create("Sales", Source{}, testTable(t, "sales_2", "sales_2"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if first.ID != "sales" {
		t.Errorf("first ID = %q, want sales", first.ID)
	}
	if second.ID != "sales-2" {
		t.Errorf("second ID = %q, want sales-2", second.ID)
	}
}

func TestStoreNeverGeneratesReservedID(t *testing.T) {
	store := NewStore()

	created, err := store.Create("preview", Source{}, testTable(t, "preview_table", "preview_table"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "preview" {
		t.Fatal("the reserved route literal was used as a dataset ID")
	}
}

func TestStoreFallsBackToDefaultName(t *testing.T) {
	created, err := NewStore().Create("   ", Source{}, testTable(t, "table", "table"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "dataset" {
		t.Errorf("Name = %q, want dataset", created.Name)
	}
}

func TestStoreRejectsInvalidTableWithoutRegistering(t *testing.T) {
	store := NewStore()

	schema, err := catalog.NewSchema(
		catalog.ColumnDef{Name: "id", Type: catalog.TypeInteger},
		catalog.ColumnDef{Name: "ID", Type: catalog.TypeInteger},
	)
	if err != nil {
		t.Fatalf("NewSchema: %v", err)
	}
	broken := &catalog.Table{
		ID:     "broken",
		Name:   "broken",
		Schema: schema,
		Rows:   []catalog.Row{{catalog.Int(1), catalog.Int(2)}},
	}

	if _, err := store.Create("broken", Source{}, broken); err == nil {
		t.Fatal("expected the invalid table to be rejected")
	}
	if got := store.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
}

func TestStoreDelete(t *testing.T) {
	store := NewStore()
	if _, err := store.Create("employees", Source{}, testTable(t, "employees", "employees")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Delete("employees"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := store.Get("employees"); ok {
		t.Error("a deleted dataset is still registered")
	}
	if got := store.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}

	if err := store.Delete("missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete(missing) = %v, want ErrNotFound", err)
	}
}

func TestSeedSamplesRegistersOneDatasetWithBothTables(t *testing.T) {
	store := NewStore()
	if err := SeedSamples(store); err != nil {
		t.Fatalf("SeedSamples: %v", err)
	}

	if got := store.Len(); got != 1 {
		t.Fatalf("Len() = %d, want 1", got)
	}

	samples, ok := store.Get("samples")
	if !ok {
		t.Fatal("samples dataset is not registered")
	}
	if got := samples.Tables.Len(); got != 2 {
		t.Errorf("table count = %d, want 2", got)
	}
	if got := samples.RowCount(); got != 12 {
		t.Errorf("row count = %d, want 12", got)
	}
	if samples.Source.Filename != "built-in" {
		t.Errorf("source filename = %q, want built-in", samples.Source.Filename)
	}
}
