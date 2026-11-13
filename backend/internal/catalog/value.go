package catalog

import (
	"math"
	"strconv"
	"strings"
)

// Type is the logical type of a column or of a value. The zero value is TypeNull,
// so the zero Value is a usable SQL NULL rather than an invalid sentinel.
type Type string

const (
	TypeNull    Type = ""
	TypeBoolean Type = "boolean"
	TypeInteger Type = "integer"
	TypeDecimal Type = "decimal"
	TypeText    Type = "text"
)

func (t Type) String() string {
	if t == TypeNull {
		return "null"
	}
	return string(t)
}

// IsColumnType reports whether t can be declared as a column type. NULL is a
// value, never a column type.
func (t Type) IsColumnType() bool {
	switch t {
	case TypeBoolean, TypeInteger, TypeDecimal, TypeText:
		return true
	default:
		return false
	}
}

// Value is a typed SQL value. NULL is represented by TypeNull and is distinct from
// the empty string, zero, and false.
type Value struct {
	typ     Type
	boolean bool
	integer int64
	decimal float64
	text    string
}

func Null() Value             { return Value{} }
func Bool(v bool) Value       { return Value{typ: TypeBoolean, boolean: v} }
func Int(v int64) Value       { return Value{typ: TypeInteger, integer: v} }
func Decimal(v float64) Value { return Value{typ: TypeDecimal, decimal: v} }
func Text(v string) Value     { return Value{typ: TypeText, text: v} }

// Type returns the value's logical type, or TypeNull for NULL.
func (v Value) Type() Type { return v.typ }

func (v Value) IsNull() bool { return v.typ == TypeNull }

// AsBool returns the boolean payload and whether the value holds a boolean. No
// coercion is performed.
func (v Value) AsBool() (bool, bool) { return v.boolean, v.typ == TypeBoolean }

// AsInt returns the integer payload and whether the value holds an integer.
func (v Value) AsInt() (int64, bool) { return v.integer, v.typ == TypeInteger }

// AsDecimal returns the decimal payload and whether the value holds a decimal.
func (v Value) AsDecimal() (float64, bool) { return v.decimal, v.typ == TypeDecimal }

// AsText returns the text payload and whether the value holds text.
func (v Value) AsText() (string, bool) { return v.text, v.typ == TypeText }

// Equals reports structural equality, under which NULL equals NULL. This is
// identity for internal use, not SQL comparison: three-valued comparison belongs
// to expression evaluation.
func (v Value) Equals(other Value) bool {
	if v.typ != other.typ {
		return false
	}
	switch v.typ {
	case TypeNull:
		return true
	case TypeBoolean:
		return v.boolean == other.boolean
	case TypeInteger:
		return v.integer == other.integer
	case TypeDecimal:
		return v.decimal == other.decimal
	case TypeText:
		return v.text == other.text
	default:
		return false
	}
}

// String renders the value the way a SQL literal would be written, so error
// messages and test failures are unambiguous.
func (v Value) String() string {
	switch v.typ {
	case TypeNull:
		return "NULL"
	case TypeBoolean:
		if v.boolean {
			return "TRUE"
		}
		return "FALSE"
	case TypeInteger:
		return strconv.FormatInt(v.integer, 10)
	case TypeDecimal:
		return formatDecimal(v.decimal)
	case TypeText:
		return "'" + strings.ReplaceAll(v.text, "'", "''") + "'"
	default:
		return "invalid"
	}
}

func formatDecimal(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	default:
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
}
