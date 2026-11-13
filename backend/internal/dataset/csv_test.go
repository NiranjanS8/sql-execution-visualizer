package dataset

import (
	"errors"
	"strings"
	"testing"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
)

func parseCSV(t *testing.T, data string, opts ImportOptions) *Parsed {
	t.Helper()

	opts.HasHeader = true
	parsed, err := ParseCSV([]byte(data), opts)
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	return parsed
}

func columnTypes(parsed *Parsed) map[string]catalog.Type {
	types := make(map[string]catalog.Type, len(parsed.Columns))
	for _, column := range parsed.Columns {
		types[column.Name] = column.Type
	}
	return types
}

func TestParseCSVInfersColumnTypes(t *testing.T) {
	parsed := parseCSV(t, "id,name,salary,active,ratio,code\n1,Ada,145000,true,0.5,007\n2,Grace,132000,false,1.25,008\n", ImportOptions{})

	want := map[string]catalog.Type{
		"id":     catalog.TypeInteger,
		"name":   catalog.TypeText,
		"salary": catalog.TypeInteger,
		"active": catalog.TypeBoolean,
		"ratio":  catalog.TypeDecimal,
		"code":   catalog.TypeText,
	}
	got := columnTypes(parsed)
	for name, wantType := range want {
		if got[name] != wantType {
			t.Errorf("column %q type = %v, want %v", name, got[name], wantType)
		}
	}

	if parsed.Table.RowCount() != 2 {
		t.Fatalf("RowCount() = %d, want 2", parsed.Table.RowCount())
	}

	row := parsed.Table.Rows[0]
	if value, _ := row[0].AsInt(); value != 1 {
		t.Errorf("id = %v, want 1", row[0])
	}
	if value, _ := row[4].AsDecimal(); value != 0.5 {
		t.Errorf("ratio = %v, want 0.5", row[4])
	}
	if value, _ := row[5].AsText(); value != "007" {
		t.Errorf("code = %q, want %q", value, "007")
	}
}

func TestParseCSVTreatsEmptyAndNullTokensAsNull(t *testing.T) {
	parsed := parseCSV(t, "id,name,salary\n1,Ada,\n2,Grace,NULL\n3,Alan,151000\n", ImportOptions{})

	if got := parsed.Columns[2].NullCount; got != 2 {
		t.Errorf("salary NullCount = %d, want 2", got)
	}
	if got := parsed.Columns[2].Nullability; got != catalog.NullabilityNullable {
		t.Errorf("salary nullability = %q, want nullable", got)
	}
	if !parsed.Table.Rows[0][2].IsNull() {
		t.Error("empty salary is not NULL")
	}
	if !parsed.Table.Rows[1][2].IsNull() {
		t.Error("NULL token is not NULL")
	}
	if got := parsed.Columns[0].Nullability; got != catalog.NullabilityNotNull {
		t.Errorf("id nullability = %q, want not_null", got)
	}
}

func TestParseCSVAcceptsCustomNullValues(t *testing.T) {
	parsed := parseCSV(t, "id,note\n1,NA\n2,hello\n3,NULL\n", ImportOptions{NullValues: []string{"NA"}})

	if !parsed.Table.Rows[0][1].IsNull() {
		t.Error("custom null token NA was not treated as NULL")
	}
	if !parsed.Table.Rows[2][1].IsNull() {
		t.Error("default null token NULL was lost when custom values were supplied")
	}
	if value, _ := parsed.Table.Rows[1][1].AsText(); value != "hello" {
		t.Errorf("note = %q, want hello", value)
	}

	joined := strings.Join(parsed.NullValues, ",")
	for _, token := range []string{"NA", "NULL"} {
		if !strings.Contains(joined, token) {
			t.Errorf("effective null values %v do not include %q", parsed.NullValues, token)
		}
	}
}

func TestParseCSVSanitizesAndDeduplicatesHeaders(t *testing.T) {
	parsed := parseCSV(t, "Full Name,full name,1st,,\nAda,Ada,x,y,z\n", ImportOptions{})

	names := parsed.Table.Schema.Names()
	want := []string{"Full_Name", "full_name_2", "_1st", "column_4", "column_5"}
	for i, expected := range want {
		if names[i] != expected {
			t.Errorf("column %d = %q, want %q", i, names[i], expected)
		}
	}

	if len(parsed.Warnings) == 0 {
		t.Error("renamed headers produced no warnings")
	}
}

func TestParseCSVWithoutHeaderGeneratesNames(t *testing.T) {
	parsed, err := ParseCSV([]byte("1,Ada\n2,Grace\n"), ImportOptions{HasHeader: false})
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}

	names := parsed.Table.Schema.Names()
	if names[0] != "column_1" || names[1] != "column_2" {
		t.Errorf("names = %v, want column_1 and column_2", names)
	}
	if parsed.Table.RowCount() != 2 {
		t.Errorf("RowCount() = %d, want 2", parsed.Table.RowCount())
	}
}

func TestParseCSVReportsRowWidthMismatch(t *testing.T) {
	_, err := ParseCSV([]byte("a,b\n1,2\n3\n"), ImportOptions{HasHeader: true})

	var importErr *ImportError
	if !errors.As(err, &importErr) {
		t.Fatalf("error = %v, want *ImportError", err)
	}
	if importErr.Line != 3 {
		t.Errorf("line = %d, want 3", importErr.Line)
	}
	if !strings.Contains(importErr.Message, "expected 2 fields, found 1") {
		t.Errorf("message = %q, want a field-count explanation", importErr.Message)
	}
}

func TestParseCSVRejectsUnusableFiles(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"empty", "", "empty"},
		{"whitespace only", " \n\n", "empty"},
		{"invalid utf8", "id,name\n1,\xff\n", "UTF-8"},
		{"nul byte", "id,name\n1,ad\x00a\n", "NUL"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseCSV([]byte(test.data), ImportOptions{HasHeader: true})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %q, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestParseCSVRejectsUnterminatedQuote(t *testing.T) {
	_, err := ParseCSV([]byte("id,note\n1,\"unclosed\n"), ImportOptions{HasHeader: true})

	var importErr *ImportError
	if !errors.As(err, &importErr) {
		t.Fatalf("error = %v, want *ImportError", err)
	}
	if importErr.Line == 0 {
		t.Error("quote error has no line number")
	}
}

func TestParseCSVEnforcesLimits(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		limits Limits
		want   string
	}{
		{
			name:   "bytes",
			data:   "id,name\n1,Ada\n",
			limits: Limits{MaxBytes: 5},
			want:   "import limit",
		},
		{
			name:   "rows",
			data:   "id\n1\n2\n3\n",
			limits: Limits{MaxRows: 2},
			want:   "more than 2 data rows",
		},
		{
			name:   "columns",
			data:   "id,name\n1,Ada\n",
			limits: Limits{MaxColumns: 1},
			want:   "2 columns",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseCSV([]byte(test.data), ImportOptions{HasHeader: true, Limits: test.limits})
			if err == nil {
				t.Fatal("expected a limit error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %q, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestParseCSVStripsBOMAndQuotedFields(t *testing.T) {
	parsed := parseCSV(t, "\xEF\xBB\xBFid,note\n1,\"hello, world\"\n", ImportOptions{})

	if parsed.Table.Schema.Names()[0] != "id" {
		t.Errorf("first column = %q, want id", parsed.Table.Schema.Names()[0])
	}
	if value, _ := parsed.Table.Rows[0][1].AsText(); value != "hello, world" {
		t.Errorf("note = %q, want %q", value, "hello, world")
	}
}

func TestParseCSVKeepsOverflowingIntegersAsText(t *testing.T) {
	parsed := parseCSV(t, "value\n9223372036854775807\n9223372036854775808\n", ImportOptions{})

	if got := parsed.Columns[0].Type; got != catalog.TypeText {
		t.Errorf("type = %v, want text when a value overflows int64", got)
	}
}

func TestParseCSVTrimsFields(t *testing.T) {
	parsed := parseCSV(t, "id,name\n  1  ,  Ada  \n", ImportOptions{})

	if value, _ := parsed.Table.Rows[0][0].AsInt(); value != 1 {
		t.Errorf("id = %v, want 1", parsed.Table.Rows[0][0])
	}
	if value, _ := parsed.Table.Rows[0][1].AsText(); value != "Ada" {
		t.Errorf("name = %q, want Ada", value)
	}
}

func TestParseCSVWarnsAboutMixedFormats(t *testing.T) {
	parsed := parseCSV(t, "value\n1\ntwo\n", ImportOptions{})

	if got := parsed.Columns[0].Type; got != catalog.TypeText {
		t.Errorf("type = %v, want text for mixed values", got)
	}
	found := false
	for _, warning := range parsed.Warnings {
		if strings.Contains(warning, "mixes value formats") {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings = %v, want a mixed-format warning", parsed.Warnings)
	}
}

func TestParseCSVHandlesHeaderOnlyFile(t *testing.T) {
	parsed := parseCSV(t, "id,name\n", ImportOptions{})

	if parsed.Table.RowCount() != 0 {
		t.Fatalf("RowCount() = %d, want 0", parsed.Table.RowCount())
	}
	for _, column := range parsed.Columns {
		if column.Type != catalog.TypeText {
			t.Errorf("column %q type = %v, want text for an empty file", column.Name, column.Type)
		}
		if column.Nullability != catalog.NullabilityUnknown {
			t.Errorf("column %q nullability = %q, want unknown", column.Name, column.Nullability)
		}
	}
}

func TestParseCSVDerivesTableNameFromFilename(t *testing.T) {
	parsed := parseCSV(t, "id\n1\n", ImportOptions{Filename: `C:\exports\my-orders.csv`})

	if parsed.Table.Name != "my_orders" {
		t.Errorf("table name = %q, want my_orders", parsed.Table.Name)
	}
	if parsed.Table.Source == nil || parsed.Table.Source.Filename != "my-orders.csv" {
		t.Errorf("source = %+v, want the base filename", parsed.Table.Source)
	}
}

func TestParseCSVHonoursTableNameOverride(t *testing.T) {
	parsed := parseCSV(t, "id\n1\n", ImportOptions{Filename: "orders.csv", TableName: "Order History"})

	if parsed.Table.Name != "Order_History" {
		t.Errorf("table name = %q, want Order_History", parsed.Table.Name)
	}
}
