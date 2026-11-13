package catalog

import "testing"

func TestZeroValueIsNull(t *testing.T) {
	var value Value

	if !value.IsNull() {
		t.Error("zero Value should be NULL")
	}
	if value.Type() != TypeNull {
		t.Errorf("type = %v, want null", value.Type())
	}
	if got := value.String(); got != "NULL" {
		t.Errorf("String() = %q, want NULL", got)
	}
}

func TestNullIsDistinctFromEmptyValues(t *testing.T) {
	null := Null()

	others := []struct {
		name  string
		value Value
	}{
		{"empty text", Text("")},
		{"zero integer", Int(0)},
		{"false boolean", Bool(false)},
		{"zero decimal", Decimal(0)},
	}

	for _, test := range others {
		name, value := test.name, test.value
		t.Run(name, func(t *testing.T) {
			if value.IsNull() {
				t.Fatalf("%s reports itself as NULL", name)
			}
			if value.Equals(null) {
				t.Errorf("%s equals NULL", name)
			}
			if null.Equals(value) {
				t.Errorf("NULL equals %s", name)
			}
			if value.Type() == TypeNull {
				t.Errorf("%s has NULL type", name)
			}
		})
	}
}

func TestConstructorsSetType(t *testing.T) {
	tests := []struct {
		name  string
		value Value
		want  Type
	}{
		{"null", Null(), TypeNull},
		{"boolean", Bool(true), TypeBoolean},
		{"integer", Int(1), TypeInteger},
		{"decimal", Decimal(1.5), TypeDecimal},
		{"text", Text("x"), TypeText},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.value.Type(); got != test.want {
				t.Errorf("type = %v, want %v", got, test.want)
			}
		})
	}
}

func TestAccessorsRejectOtherTypes(t *testing.T) {
	value := Int(42)

	if _, ok := value.AsBool(); ok {
		t.Error("AsBool accepted an integer")
	}
	if _, ok := value.AsDecimal(); ok {
		t.Error("AsDecimal accepted an integer")
	}
	if _, ok := value.AsText(); ok {
		t.Error("AsText accepted an integer")
	}

	got, ok := value.AsInt()
	if !ok {
		t.Fatal("AsInt rejected an integer")
	}
	if got != 42 {
		t.Errorf("AsInt() = %d, want 42", got)
	}
}

func TestEquals(t *testing.T) {
	tests := []struct {
		name string
		a    Value
		b    Value
		want bool
	}{
		{"same integer", Int(7), Int(7), true},
		{"different integers", Int(7), Int(8), false},
		{"same text", Text("Ada"), Text("Ada"), true},
		{"text is case sensitive", Text("Ada"), Text("ada"), false},
		{"null equals null", Null(), Null(), true},
		{"integer is not decimal", Int(1), Decimal(1), false},
		{"empty text is not null", Text(""), Null(), false},
		{"zero is not false", Int(0), Bool(false), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.a.Equals(test.b); got != test.want {
				t.Errorf("%s.Equals(%s) = %v, want %v", test.a, test.b, got, test.want)
			}
		})
	}
}

func TestStringRendering(t *testing.T) {
	tests := []struct {
		value Value
		want  string
	}{
		{Null(), "NULL"},
		{Bool(true), "TRUE"},
		{Bool(false), "FALSE"},
		{Int(-42), "-42"},
		{Decimal(3.14), "3.14"},
		{Decimal(4_200_000), "4200000"},
		{Text("Ada"), "'Ada'"},
		{Text("O'Brien"), "'O''Brien'"},
		{Text(""), "''"},
	}

	for _, test := range tests {
		if got := test.value.String(); got != test.want {
			t.Errorf("String() = %q, want %q", got, test.want)
		}
	}
}
