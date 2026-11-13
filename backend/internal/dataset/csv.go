package dataset

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
)

// Limits bounds an import so a malformed or hostile file cannot exhaust memory.
// A zero field falls back to the default for that field.
type Limits struct {
	MaxBytes   int64
	MaxRows    int
	MaxColumns int
}

// DefaultLimits is the import budget for a single CSV file.
func DefaultLimits() Limits {
	return Limits{
		MaxBytes:   5 << 20, // 5 MiB
		MaxRows:    100_000,
		MaxColumns: 100,
	}
}

func (l Limits) normalized() Limits {
	defaults := DefaultLimits()
	if l.MaxBytes <= 0 {
		l.MaxBytes = defaults.MaxBytes
	}
	if l.MaxRows <= 0 {
		l.MaxRows = defaults.MaxRows
	}
	if l.MaxColumns <= 0 {
		l.MaxColumns = defaults.MaxColumns
	}
	return l
}

// ImportOptions controls how a CSV file becomes a table.
type ImportOptions struct {
	// Filename is recorded as the table's source and supplies the default table
	// name when TableName is empty.
	Filename string
	// TableName overrides the name derived from Filename.
	TableName string
	// HasHeader treats the first record as column names.
	HasHeader bool
	// NullValues are additional field values treated as NULL. Empty fields are
	// always NULL. Nil means the defaults: NULL and \N.
	NullValues []string
	// Limits bounds the import. The zero value means DefaultLimits.
	Limits Limits
}

// ColumnSummary describes one inferred column of a parsed file.
type ColumnSummary struct {
	Name        string
	Type        catalog.Type
	Nullability catalog.Nullability
	NullCount   int
}

// Parsed is the result of reading a CSV file. Nothing is registered yet; the
// caller decides whether to commit the table to a dataset.
type Parsed struct {
	Table      *catalog.Table
	Columns    []ColumnSummary
	Bytes      int64
	Warnings   []string
	NullValues []string
}

// ImportError reports a user-fixable problem in the uploaded file. Line and
// Column are 1-based source positions, and are zero when the problem has no
// single position.
type ImportError struct {
	Line    int
	Column  int
	Message string
}

func (e *ImportError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Message)
	}
	return e.Message
}

// defaultNullValues are the field values treated as NULL in addition to the
// empty field, which is always NULL.
var defaultNullValues = []string{"NULL", `\N`}

type csvRecord struct {
	fields []string
	line   int
}

// ParseCSV reads a UTF-8 CSV file and infers a table schema conservatively.
//
// Inference is deterministic and only widens: a column becomes the first of
// integer, decimal, boolean, or text that every non-NULL value satisfies.
// Values that look like identifiers (leading zeros) or that overflow int64 stay
// text rather than being silently converted.
func ParseCSV(data []byte, opts ImportOptions) (*Parsed, error) {
	limits := opts.Limits.normalized()
	byteCount := int64(len(data))

	if byteCount > limits.MaxBytes {
		return nil, &ImportError{Message: fmt.Sprintf(
			"file is %s, above the %s import limit", formatBytes(byteCount), formatBytes(limits.MaxBytes))}
	}

	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, &ImportError{Message: "file is empty"}
	}
	if !utf8.Valid(data) {
		return nil, &ImportError{Message: "file is not valid UTF-8; export it as UTF-8 CSV"}
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, &ImportError{Message: "file contains a NUL byte; only text CSV files are supported"}
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = false

	first, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return nil, &ImportError{Message: "file is empty"}
	}
	if err != nil {
		return nil, csvReadError(err)
	}

	width := len(first)
	if width == 0 {
		return nil, &ImportError{Line: 1, Message: "the first record has no fields"}
	}
	if width > limits.MaxColumns {
		return nil, &ImportError{Line: 1, Message: fmt.Sprintf(
			"the file has %d columns; the import limit is %d", width, limits.MaxColumns)}
	}

	firstLine, _ := reader.FieldPos(0)

	headers := make([]string, width)
	records := make([]csvRecord, 0, 64)
	if opts.HasHeader {
		copy(headers, first)
	} else {
		for i := range headers {
			headers[i] = fmt.Sprintf("column_%d", i+1)
		}
		records = append(records, csvRecord{fields: first, line: firstLine})
	}

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, csvReadError(err)
		}

		line, _ := reader.FieldPos(0)
		if len(record) != width {
			return nil, &ImportError{
				Line:    line,
				Column:  len(record) + 1,
				Message: fmt.Sprintf("expected %d fields, found %d", width, len(record)),
			}
		}
		if len(records) >= limits.MaxRows {
			return nil, &ImportError{
				Line: line,
				Message: fmt.Sprintf(
					"the file has more than %d data rows; the import limit is %d", limits.MaxRows, limits.MaxRows),
			}
		}

		records = append(records, csvRecord{fields: record, line: line})
	}

	names, warnings := normalizeHeaders(headers)
	effectiveNulls := effectiveNullValues(opts.NullValues)
	nulls := nullSet(effectiveNulls)

	states := make([]columnState, width)
	for i := range states {
		states[i] = columnState{integer: true, decimal: true, boolean: true}
	}
	nullCounts := make([]int, width)

	for _, record := range records {
		for i, raw := range record.fields {
			field := strings.TrimSpace(raw)
			if isNull(field, nulls) {
				nullCounts[i]++
				continue
			}
			states[i].observe(field)
		}
	}

	types := make([]catalog.Type, width)
	defs := make([]catalog.ColumnDef, width)
	for i := range defs {
		types[i] = states[i].columnType()
		nullability := catalog.NullabilityUnknown
		if len(records) > 0 {
			if nullCounts[i] > 0 {
				nullability = catalog.NullabilityNullable
			} else {
				nullability = catalog.NullabilityNotNull
			}
		}
		defs[i] = catalog.ColumnDef{Name: names[i], Type: types[i], Nullability: nullability}

		if types[i] == catalog.TypeText && states[i].sawScalar {
			warnings = append(warnings, fmt.Sprintf(
				"column %q mixes value formats and was imported as text", names[i]))
		}
	}

	schema, err := catalog.NewSchema(defs...)
	if err != nil {
		return nil, &ImportError{Message: err.Error()}
	}

	rows := make([]catalog.Row, len(records))
	for r, record := range records {
		row := make(catalog.Row, width)
		for i, raw := range record.fields {
			field := strings.TrimSpace(raw)
			if isNull(field, nulls) {
				row[i] = catalog.Null()
				continue
			}
			value, err := parseValue(types[i], field)
			if err != nil {
				return nil, &ImportError{
					Line:    record.line,
					Column:  i + 1,
					Message: fmt.Sprintf("column %q: %s", names[i], err.Error()),
				}
			}
			row[i] = value
		}
		rows[r] = row
	}

	tableName := tableNameFor(opts.TableName, opts.Filename)
	table := &catalog.Table{
		ID:     catalog.TableID(tableName),
		Name:   tableName,
		Schema: schema,
		Rows:   rows,
	}
	if opts.Filename != "" {
		table.Source = &catalog.Source{Filename: BaseFilename(opts.Filename), ImportedAt: time.Now().UTC()}
	}
	if err := table.Validate(); err != nil {
		return nil, &ImportError{Message: err.Error()}
	}

	summaries := make([]ColumnSummary, width)
	for i := range summaries {
		summaries[i] = ColumnSummary{
			Name:        names[i],
			Type:        types[i],
			Nullability: schema.ColumnAt(i).Nullability,
			NullCount:   nullCounts[i],
		}
	}

	return &Parsed{
		Table:      table,
		Columns:    summaries,
		Bytes:      byteCount,
		Warnings:   warnings,
		NullValues: effectiveNulls,
	}, nil
}

type columnState struct {
	seen      bool
	integer   bool
	decimal   bool
	boolean   bool
	sawScalar bool
}

func (s *columnState) observe(field string) {
	s.seen = true
	if s.integer && !isIntegerLiteral(field) {
		s.integer = false
	}
	if s.decimal && !isDecimalLiteral(field) {
		s.decimal = false
	}
	if s.boolean && !isBooleanLiteral(field) {
		s.boolean = false
	}
	if isIntegerLiteral(field) || isDecimalLiteral(field) || isBooleanLiteral(field) {
		s.sawScalar = true
	}
}

func (s columnState) columnType() catalog.Type {
	switch {
	case !s.seen:
		return catalog.TypeText
	case s.integer:
		return catalog.TypeInteger
	case s.decimal:
		return catalog.TypeDecimal
	case s.boolean:
		return catalog.TypeBoolean
	default:
		return catalog.TypeText
	}
}

func normalizeHeaders(headers []string) ([]string, []string) {
	names := make([]string, len(headers))
	var warnings []string

	for i, header := range headers {
		trimmed := strings.TrimSpace(header)
		fallback := fmt.Sprintf("column_%d", i+1)
		names[i] = sanitizeIdentifier(trimmed, fallback)

		switch {
		case trimmed == "":
			warnings = append(warnings, fmt.Sprintf(
				"column %d has no header and was named %q", i+1, names[i]))
		case names[i] != trimmed:
			warnings = append(warnings, fmt.Sprintf(
				"column %q was imported as %q", trimmed, names[i]))
		}
	}

	deduped := dedupeNames(names)
	for i := range deduped {
		if deduped[i] != names[i] {
			warnings = append(warnings, fmt.Sprintf(
				"duplicate column %q was renamed to %q", names[i], deduped[i]))
		}
	}

	return deduped, warnings
}

func effectiveNullValues(values []string) []string {
	set := make(map[string]struct{}, len(values)+1)
	set[""] = struct{}{}
	for _, value := range defaultNullValues {
		set[value] = struct{}{}
	}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}

	effective := make([]string, 0, len(set))
	for value := range set {
		effective = append(effective, value)
	}
	slices.Sort(effective)
	return effective
}

func nullSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[strings.ToLower(value)] = struct{}{}
	}
	return set
}

func isNull(field string, nulls map[string]struct{}) bool {
	_, ok := nulls[strings.ToLower(field)]
	return ok
}

func isIntegerLiteral(field string) bool {
	if field == "" || hasLeadingZero(field) {
		return false
	}

	digits := field
	if digits[0] == '-' {
		digits = digits[1:]
	}
	if digits == "" {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}

	_, err := strconv.ParseInt(field, 10, 64)
	return err == nil
}

func isDecimalLiteral(field string) bool {
	if field == "" || hasLeadingZero(field) {
		return false
	}

	for _, r := range field {
		switch {
		case r >= '0' && r <= '9':
		case r == '+', r == '-', r == '.', r == 'e', r == 'E':
		default:
			return false
		}
	}

	// A pure integer that overflows int64 must not fall through to decimal:
	// float64 cannot represent it exactly, so it stays text.
	if !strings.ContainsAny(field, ".eE") {
		if _, err := strconv.ParseInt(field, 10, 64); err != nil {
			return false
		}
	}

	value, err := strconv.ParseFloat(field, 64)
	if err != nil {
		return false
	}
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// hasLeadingZero rejects identifier-like numerics such as "007" so they stay text.
func hasLeadingZero(field string) bool {
	digits := field
	if len(digits) > 0 && (digits[0] == '+' || digits[0] == '-') {
		digits = digits[1:]
	}
	return len(digits) > 1 && digits[0] == '0' && digits[1] >= '0' && digits[1] <= '9'
}

func isBooleanLiteral(field string) bool {
	return strings.EqualFold(field, "true") || strings.EqualFold(field, "false")
}

func parseValue(typ catalog.Type, field string) (catalog.Value, error) {
	switch typ {
	case catalog.TypeInteger:
		value, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return catalog.Null(), fmt.Errorf("%q is not an integer", field)
		}
		return catalog.Int(value), nil
	case catalog.TypeDecimal:
		value, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return catalog.Null(), fmt.Errorf("%q is not a decimal number", field)
		}
		return catalog.Decimal(value), nil
	case catalog.TypeBoolean:
		return catalog.Bool(strings.EqualFold(field, "true")), nil
	default:
		return catalog.Text(field), nil
	}
}

func csvReadError(err error) error {
	var parseErr *csv.ParseError
	if errors.As(err, &parseErr) {
		message := parseErr.Err.Error()
		switch {
		case errors.Is(parseErr.Err, csv.ErrQuote):
			message = "unterminated quoted field"
		case errors.Is(parseErr.Err, csv.ErrBareQuote):
			message = "a quote appeared inside an unquoted field"
		}
		return &ImportError{Line: parseErr.Line, Column: parseErr.Column, Message: message}
	}
	return &ImportError{Message: err.Error()}
}
