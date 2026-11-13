package catalog

import "testing"

func TestSampleTablesValidate(t *testing.T) {
	tables := SampleTables()

	if len(tables) != 2 {
		t.Fatalf("SampleTables() returned %d tables, want 2", len(tables))
	}
	for _, table := range tables {
		if err := table.Validate(); err != nil {
			t.Errorf("%s: %v", table.ID, err)
		}
	}
}

func TestNewWithSamplesRegistersEveryTable(t *testing.T) {
	store := NewWithSamples()

	if got, want := store.Len(), len(SampleTables()); got != want {
		t.Fatalf("Len() = %d, want %d", got, want)
	}

	employees, ok := store.Get("employees")
	if !ok {
		t.Fatal("employees is not registered")
	}
	if got := employees.RowCount(); got != 8 {
		t.Errorf("employees RowCount() = %d, want 8", got)
	}
	if got := employees.ColumnCount(); got != 5 {
		t.Errorf("employees ColumnCount() = %d, want 5", got)
	}

	departments, ok := store.Get("departments")
	if !ok {
		t.Fatal("departments is not registered")
	}
	if got := departments.ColumnCount(); got != 4 {
		t.Errorf("departments ColumnCount() = %d, want 4", got)
	}
}

func TestSampleDataExercisesNulls(t *testing.T) {
	employees, ok := NewWithSamples().Get("employees")
	if !ok {
		t.Fatal("employees is not registered")
	}

	var nullSalary, nullDepartment bool
	for _, row := range employees.Rows {
		nullSalary = nullSalary || row[2].IsNull()
		nullDepartment = nullDepartment || row[3].IsNull()
	}

	if !nullSalary {
		t.Error("sample employees contains no NULL salary")
	}
	if !nullDepartment {
		t.Error("sample employees contains no NULL dept_id")
	}
}

func TestSampleTableOrderIsStable(t *testing.T) {
	first := NewWithSamples().List()
	second := NewWithSamples().List()

	if len(first) != len(second) {
		t.Fatalf("list lengths differ: %d and %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Errorf("position %d: %q and %q", i, first[i].ID, second[i].ID)
		}
	}
}
