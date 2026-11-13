package catalog

// SampleTables returns the built-in datasets: a small company with departments and
// employees. They let the product be explored without importing a file first.
//
// The data is static, so a construction failure is a programming error and panics.
// Tests assert that every built-in table validates.
func SampleTables() []*Table {
	return []*Table{
		mustTable(TableID("departments"), "departments", []ColumnDef{
			{Name: "id", Type: TypeInteger, Nullability: NullabilityNotNull},
			{Name: "name", Type: TypeText, Nullability: NullabilityNotNull},
			{Name: "headcount", Type: TypeInteger, Nullability: NullabilityNotNull},
			{Name: "budget", Type: TypeDecimal, Nullability: NullabilityNotNull},
		}, []Row{
			{Int(10), Text("Engineering"), Int(42), Decimal(4_200_000)},
			{Int(20), Text("Research"), Int(18), Decimal(1_850_000)},
			{Int(30), Text("Sales"), Int(27), Decimal(2_400_000)},
			{Int(40), Text("Support"), Int(12), Decimal(950_000)},
		}),
		mustTable(TableID("employees"), "employees", []ColumnDef{
			{Name: "id", Type: TypeInteger, Nullability: NullabilityNotNull},
			{Name: "name", Type: TypeText, Nullability: NullabilityNotNull},
			{Name: "salary", Type: TypeInteger, Nullability: NullabilityNullable},
			{Name: "dept_id", Type: TypeInteger, Nullability: NullabilityNullable},
			{Name: "active", Type: TypeBoolean, Nullability: NullabilityNotNull},
		}, []Row{
			{Int(1), Text("Ada Lovelace"), Int(145_000), Int(10), Bool(true)},
			{Int(2), Text("Grace Hopper"), Int(132_000), Int(10), Bool(true)},
			{Int(3), Text("Alan Turing"), Int(151_000), Int(20), Bool(true)},
			{Int(4), Text("Katherine Johnson"), Int(118_000), Int(20), Bool(false)},
			{Int(5), Text("Donald Knuth"), Int(127_000), Int(10), Bool(true)},
			{Int(6), Text("Margaret Hamilton"), Int(124_000), Int(10), Bool(true)},
			{Int(7), Text("Edsger Dijkstra"), Null(), Int(20), Bool(true)},
			{Int(8), Text("Barbara Liskov"), Int(109_000), Null(), Bool(true)},
		}),
	}
}

// NewWithSamples returns a catalog seeded with SampleTables.
func NewWithSamples() *Catalog {
	store := New()
	for _, table := range SampleTables() {
		if err := store.Create(table); err != nil {
			panic("catalog: seeding sample table " + string(table.ID) + ": " + err.Error())
		}
	}
	return store
}

func mustTable(id TableID, name string, defs []ColumnDef, rows []Row) *Table {
	schema, err := NewSchema(defs...)
	if err != nil {
		panic("catalog: built-in table " + name + ": " + err.Error())
	}

	table := &Table{ID: id, Name: name, Schema: schema, Rows: rows}
	if err := table.Validate(); err != nil {
		panic("catalog: built-in table " + name + ": " + err.Error())
	}

	return table
}
