// Package validate resolves a parsed SELECT against the catalog.
//
// It binds every table and column reference to a concrete catalog table and
// column, expands wildcards, derives output names and types, and rejects
// references that are unknown, ambiguous, or used with incompatible types. Type
// rules themselves live in internal/eval; this package applies them to a whole
// statement so execution only sees a checked query.
package validate

import (
	"fmt"
	"strings"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/eval"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

// maxSuggestions bounds how many names an error message lists.
const maxSuggestions = 8

// Source is one table in scope. Qualifier is the name a query must use to
// reference it: the alias when present, otherwise the table name.
type Source struct {
	Table *catalog.Table
	Alias string
}

func (s Source) Qualifier() string {
	if s.Alias != "" {
		return s.Alias
	}
	return s.Table.Name
}

// OutputColumn is one column of the query result. Type is the inferred static
// type of Expr; it is TypeNull when the expression is the NULL literal.
type OutputColumn struct {
	Name string
	Type catalog.Type
	Expr sqlparse.Expression
}

// Query is a validated single-table SELECT. Every ColumnRef reachable from
// Output, Where, and OrderBy carries a non-nil Resolved binding.
type Query struct {
	Source  Source
	Output  []OutputColumn
	Where   sqlparse.Expression // nil when the statement has no WHERE clause
	OrderBy []sqlparse.OrderItem
	Limit   *int // nil when the statement has no LIMIT clause
}

// Resolve validates stmt against the tables in the catalog and returns the bound
// query. The first failure is reported; validation does not collect every error.
func Resolve(stmt *sqlparse.Select, tables *catalog.Catalog) (*Query, error) {
	table, ok := tables.Lookup(stmt.From.Name)
	if !ok {
		return nil, unknownTable(stmt.From, tables.Names())
	}

	source := Source{Table: table, Alias: stmt.From.Alias}
	sources := []Source{source}

	output, err := resolveProjection(stmt.Items, sources)
	if err != nil {
		return nil, err
	}

	if stmt.Where != nil {
		if err := bind(stmt.Where, sources); err != nil {
			return nil, err
		}
		if err := checkPredicate(stmt.Where); err != nil {
			return nil, err
		}
	}

	orderBy, err := resolveOrderBy(stmt.OrderBy, sources, output)
	if err != nil {
		return nil, err
	}

	return &Query{
		Source:  source,
		Output:  output,
		Where:   stmt.Where,
		OrderBy: orderBy,
		Limit:   stmt.Limit,
	}, nil
}

func resolveProjection(items []sqlparse.SelectItem, sources []Source) ([]OutputColumn, error) {
	output := make([]OutputColumn, 0, len(items))

	for _, item := range items {
		if star, ok := item.Expr.(*sqlparse.Star); ok {
			expanded, err := expandStar(star, sources)
			if err != nil {
				return nil, err
			}
			output = append(output, expanded...)
			continue
		}

		if err := bind(item.Expr, sources); err != nil {
			return nil, err
		}

		typ, err := eval.Infer(item.Expr)
		if err != nil {
			return nil, err
		}

		name := item.Alias
		if name == "" {
			if ref, ok := item.Expr.(*sqlparse.ColumnRef); ok && ref.Resolved != nil {
				name = ref.Resolved.Column.Name
			}
		}
		output = append(output, OutputColumn{Name: name, Type: typ, Expr: item.Expr})
	}

	for i := range output {
		if output[i].Name == "" {
			output[i].Name = fmt.Sprintf("expr_%d", i+1)
		}
	}

	return output, nil
}

func expandStar(star *sqlparse.Star, sources []Source) ([]OutputColumn, error) {
	selected := sources
	if star.Table != "" {
		source, err := qualifiedSource(sources, star.Table, star.Pos)
		if err != nil {
			return nil, err
		}
		selected = []Source{source}
	}

	var output []OutputColumn
	for _, source := range selected {
		for index, column := range source.Table.Schema.Columns() {
			ref := &sqlparse.ColumnRef{
				Table: star.Table,
				Name:  column.Name,
				Pos:   star.Pos,
				Resolved: &sqlparse.ResolvedColumn{
					Source: source.Table,
					Column: column,
					Index:  index,
				},
			}
			output = append(output, OutputColumn{Name: column.Name, Type: column.Type, Expr: ref})
		}
	}

	return output, nil
}

func resolveOrderBy(items []sqlparse.OrderItem, sources []Source, output []OutputColumn) ([]sqlparse.OrderItem, error) {
	for _, item := range items {
		if literal, ok := item.Expr.(*sqlparse.Literal); ok && literal.Value.Type() == catalog.TypeInteger {
			return nil, &sqlparse.Error{
				Code:    sqlparse.CodeUnsupportedSyntax,
				Message: "ORDER BY by position is not supported; name the column instead.",
				Line:    item.Pos.Line,
				Column:  item.Pos.Column,
			}
		}

		if err := bind(item.Expr, sources); err != nil {
			if alias := outputAlias(item.Expr, output); alias != "" {
				return nil, &sqlparse.Error{
					Code:    sqlparse.CodeUnsupportedSyntax,
					Message: fmt.Sprintf("ORDER BY cannot reference the output alias %q yet; order by the source column instead.", alias),
					Line:    item.Pos.Line,
					Column:  item.Pos.Column,
				}
			}
			return nil, err
		}

		if _, err := eval.Infer(item.Expr); err != nil {
			return nil, err
		}
	}

	return items, nil
}

// checkPredicate requires a WHERE clause to be boolean. A NULL predicate is
// allowed and behaves as unknown, so no row matches it.
func checkPredicate(expr sqlparse.Expression) error {
	typ, err := eval.Infer(expr)
	if err != nil {
		return err
	}
	if typ != catalog.TypeBoolean && typ != catalog.TypeNull {
		return &eval.Error{
			Code:    eval.CodeTypeError,
			Message: fmt.Sprintf("WHERE requires a boolean predicate, found %s.", typ),
			Line:    expr.Position().Line,
			Column:  expr.Position().Column,
		}
	}
	return nil
}

func outputAlias(expr sqlparse.Expression, output []OutputColumn) string {
	ref, ok := expr.(*sqlparse.ColumnRef)
	if !ok || ref.Table != "" {
		return ""
	}
	for _, column := range output {
		if strings.EqualFold(column.Name, ref.Name) {
			return column.Name
		}
	}
	return ""
}

// bind walks an expression and attaches a resolved column to every reference.
func bind(expr sqlparse.Expression, sources []Source) error {
	switch node := expr.(type) {
	case *sqlparse.ColumnRef:
		resolved, err := bindColumn(sources, node)
		if err != nil {
			return err
		}
		node.Resolved = resolved
		return nil

	case *sqlparse.Literal:
		return nil

	case *sqlparse.Unary:
		return bind(node.Operand, sources)

	case *sqlparse.Binary:
		if err := bind(node.Left, sources); err != nil {
			return err
		}
		return bind(node.Right, sources)

	case *sqlparse.IsNull:
		return bind(node.Operand, sources)

	case *sqlparse.Star:
		return &sqlparse.Error{
			Code:    sqlparse.CodeUnsupportedSyntax,
			Message: "A wildcard is only allowed in the SELECT list.",
			Line:    node.Pos.Line,
			Column:  node.Pos.Column,
		}

	default:
		return fmt.Errorf("validate: unsupported expression node %T", expr)
	}
}

// columnMatch is one source/column pair found while resolving a reference.
type columnMatch struct {
	source Source
	column catalog.Column
	index  int
}

func bindColumn(sources []Source, ref *sqlparse.ColumnRef) (*sqlparse.ResolvedColumn, error) {
	if ref.Table != "" {
		source, err := qualifiedSource(sources, ref.Table, ref.Pos)
		if err != nil {
			return nil, err
		}

		column, index, ok := findColumn(source.Table, ref.Name)
		if !ok {
			return nil, unknownColumn(ref, []Source{source})
		}
		return &sqlparse.ResolvedColumn{Source: source.Table, Column: column, Index: index}, nil
	}

	matches := make([]columnMatch, 0, len(sources))
	for _, source := range sources {
		if column, index, ok := findColumn(source.Table, ref.Name); ok {
			matches = append(matches, columnMatch{source: source, column: column, index: index})
		}
	}

	switch len(matches) {
	case 0:
		return nil, unknownColumn(ref, sources)
	case 1:
		return &sqlparse.ResolvedColumn{
			Source: matches[0].source.Table,
			Column: matches[0].column,
			Index:  matches[0].index,
		}, nil
	default:
		return nil, ambiguousColumn(ref, matches)
	}
}

func qualifiedSource(sources []Source, qualifier string, pos sqlparse.Position) (Source, error) {
	matches := make([]Source, 0, 1)
	for _, source := range sources {
		if strings.EqualFold(source.Qualifier(), qualifier) {
			matches = append(matches, source)
		}
	}

	switch len(matches) {
	case 0:
		return Source{}, unknownQualifier(qualifier, pos, sources)
	case 1:
		return matches[0], nil
	default:
		return Source{}, fmt.Errorf("validate: duplicate qualifier %q in scope", qualifier)
	}
}

func findColumn(table *catalog.Table, name string) (catalog.Column, int, bool) {
	for index, column := range table.Schema.Columns() {
		if strings.EqualFold(column.Name, name) {
			return column, index, true
		}
	}
	return catalog.Column{}, 0, false
}

func unknownTable(ref sqlparse.TableRef, available []string) *sqlparse.Error {
	message := fmt.Sprintf("Table %q does not exist.", ref.Name)
	if hint := listHint("Available tables", available); hint != "" {
		message += " " + hint
	}
	return &sqlparse.Error{
		Code:    sqlparse.CodeUnknownTable,
		Message: message,
		Line:    ref.Pos.Line,
		Column:  ref.Pos.Column,
	}
}

func unknownQualifier(qualifier string, pos sqlparse.Position, sources []Source) *sqlparse.Error {
	qualifiers := make([]string, len(sources))
	for i, source := range sources {
		qualifiers[i] = source.Qualifier()
	}

	message := fmt.Sprintf("Table or alias %q is not in this query.", qualifier)
	if hint := listHint("Available names", qualifiers); hint != "" {
		message += " " + hint
	}
	return &sqlparse.Error{
		Code:    sqlparse.CodeUnknownTable,
		Message: message,
		Line:    pos.Line,
		Column:  pos.Column,
	}
}

func unknownColumn(ref *sqlparse.ColumnRef, sources []Source) *sqlparse.Error {
	var message string
	switch len(sources) {
	case 1:
		message = fmt.Sprintf("Column %q does not exist in table %q.", ref.Name, sources[0].Qualifier())
		if hint := listHint("Available columns", sources[0].Table.Schema.Names()); hint != "" {
			message += " " + hint
		}
	default:
		message = fmt.Sprintf("Column %q does not exist in any table in this query.", ref.Name)
	}

	return &sqlparse.Error{
		Code:    sqlparse.CodeUnknownColumn,
		Message: message,
		Line:    ref.Pos.Line,
		Column:  ref.Pos.Column,
	}
}

func ambiguousColumn(ref *sqlparse.ColumnRef, matches []columnMatch) *sqlparse.Error {
	qualifiers := make([]string, len(matches))
	for i, match := range matches {
		qualifiers[i] = match.source.Qualifier()
	}

	return &sqlparse.Error{
		Code:    sqlparse.CodeAmbiguousColumn,
		Message: fmt.Sprintf("Column %q is ambiguous; qualify it with one of: %s.", ref.Name, strings.Join(qualifiers, ", ")),
		Line:    ref.Pos.Line,
		Column:  ref.Pos.Column,
	}
}

func listHint(label string, values []string) string {
	if len(values) == 0 {
		return ""
	}

	shown := values
	suffix := ""
	if len(shown) > maxSuggestions {
		shown = shown[:maxSuggestions]
		suffix = fmt.Sprintf(", and %d more", len(values)-maxSuggestions)
	}

	return fmt.Sprintf("%s: %s%s.", label, strings.Join(shown, ", "), suffix)
}
