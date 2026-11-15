// Package planner turns a validated single-table query into a logical plan and
// the operator chain that runs it.
//
// The plan is data: each node names its kind, its input, and the configuration
// the executor needs. It is deliberately independent of JSON, React Flow, and
// the HTTP layer. Phase 7 instruments the plan in internal/metrics; Phase 9 maps
// plan nodes to graph nodes.
//
// The planner never re-parses or re-resolves SQL. It consumes a
// *validate.Query, whose expressions are already bound and type-checked.
package planner

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

// Kind names a logical operator.
type Kind string

const (
	KindScan    Kind = "scan"
	KindFilter  Kind = "filter"
	KindSort    Kind = "sort"
	KindProject Kind = "project"
	KindLimit   Kind = "limit"
)

// Node is one logical operator. Schema is the shape of the rows the node emits,
// and Children lists its inputs in data-flow order.
type Node interface {
	Kind() Kind
	Children() []Node
	Schema() catalog.Schema
	Describe() string
}

// Scan reads every row of a table.
type Scan struct {
	Table *catalog.Table
}

func NewScan(table *catalog.Table) *Scan { return &Scan{Table: table} }

func (n *Scan) Kind() Kind             { return KindScan }
func (n *Scan) Children() []Node       { return nil }
func (n *Scan) Schema() catalog.Schema { return n.Table.Schema }
func (n *Scan) Describe() string       { return n.Table.Name }

// Filter drops rows for which Predicate is not TRUE.
type Filter struct {
	Input     Node
	Predicate sqlparse.Expression
}

func NewFilter(input Node, predicate sqlparse.Expression) *Filter {
	return &Filter{Input: input, Predicate: predicate}
}

func (n *Filter) Kind() Kind             { return KindFilter }
func (n *Filter) Children() []Node       { return []Node{n.Input} }
func (n *Filter) Schema() catalog.Schema { return n.Input.Schema() }
func (n *Filter) Describe() string       { return formatExpr(n.Predicate) }

// SortTerm is one ORDER BY term.
type SortTerm struct {
	Expr sqlparse.Expression
	Desc bool
}

// Sort orders its input stably. The executor puts NULLs last in both directions.
type Sort struct {
	Input Node
	Terms []SortTerm
}

func NewSort(input Node, terms ...SortTerm) *Sort {
	return &Sort{Input: input, Terms: terms}
}

func (n *Sort) Kind() Kind             { return KindSort }
func (n *Sort) Children() []Node       { return []Node{n.Input} }
func (n *Sort) Schema() catalog.Schema { return n.Input.Schema() }

func (n *Sort) Describe() string {
	terms := make([]string, len(n.Terms))
	for i, term := range n.Terms {
		direction := "ASC"
		if term.Desc {
			direction = "DESC"
		}
		terms[i] = formatExpr(term.Expr) + " " + direction
	}
	return strings.Join(terms, ", ")
}

// Column is one output column of a projection. Type comes from validation, so
// the plan does not re-infer it.
type Column struct {
	Name string
	Type catalog.Type
	Expr sqlparse.Expression
}

// Project evaluates one expression per output column.
type Project struct {
	Input   Node
	Columns []Column

	schema catalog.Schema
}

func NewProject(input Node, columns ...Column) (*Project, error) {
	if len(columns) == 0 {
		return nil, fmt.Errorf("planner: projection must declare at least one column")
	}

	defs := make([]catalog.ColumnDef, len(columns))
	for i, column := range columns {
		defs[i] = catalog.ColumnDef{Name: column.Name, Type: column.Type}
	}
	schema, err := catalog.NewResultSchema(defs...)
	if err != nil {
		return nil, fmt.Errorf("planner: projection schema: %w", err)
	}

	return &Project{Input: input, Columns: columns, schema: schema}, nil
}

func (n *Project) Kind() Kind             { return KindProject }
func (n *Project) Children() []Node       { return []Node{n.Input} }
func (n *Project) Schema() catalog.Schema { return n.schema }

func (n *Project) Describe() string {
	columns := make([]string, len(n.Columns))
	for i, column := range n.Columns {
		columns[i] = describeColumn(column)
	}
	return strings.Join(columns, ", ")
}

// Limit emits at most Count rows.
type Limit struct {
	Input Node
	Count int
}

func NewLimit(input Node, count int) (*Limit, error) {
	if count < 0 {
		return nil, fmt.Errorf("planner: LIMIT must not be negative, got %d", count)
	}
	return &Limit{Input: input, Count: count}, nil
}

func (n *Limit) Kind() Kind             { return KindLimit }
func (n *Limit) Children() []Node       { return []Node{n.Input} }
func (n *Limit) Schema() catalog.Schema { return n.Input.Schema() }
func (n *Limit) Describe() string       { return strconv.Itoa(n.Count) }

// Plan is a logical operator tree. Root is the node whose rows are the result.
type Plan struct {
	Root Node
}

func describeColumn(column Column) string {
	if ref, ok := column.Expr.(*sqlparse.ColumnRef); ok && ref.Table == "" && ref.Name == column.Name {
		return column.Name
	}
	return formatExpr(column.Expr) + " AS " + column.Name
}
