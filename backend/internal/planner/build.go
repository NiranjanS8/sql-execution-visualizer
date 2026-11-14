package planner

import (
	"fmt"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/eval"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/validate"
)

// Build turns a validated query into a logical plan. The operator order is
// Scan → Filter → Sort → Project → Limit:
//
//   - WHERE runs before sorting so the sort sees fewer rows.
//   - ORDER BY resolves source columns, so sorting must run before projection.
//   - Projection builds the result schema.
//   - LIMIT applies to the projected result.
func Build(query *validate.Query) (*Plan, error) {
	if query == nil {
		return nil, fmt.Errorf("planner: query is nil")
	}
	if query.Source.Table == nil {
		return nil, fmt.Errorf("planner: query has no source table")
	}

	var node Node = NewScan(query.Source.Table)

	if query.Where != nil {
		node = NewFilter(node, query.Where)
	}

	if len(query.OrderBy) > 0 {
		terms := make([]SortTerm, len(query.OrderBy))
		for i, item := range query.OrderBy {
			terms[i] = SortTerm{Expr: item.Expr, Desc: item.Desc}
		}
		node = NewSort(node, terms...)
	}

	columns := make([]Column, len(query.Output))
	for i, output := range query.Output {
		columns[i] = Column{Name: output.Name, Type: output.Type, Expr: output.Expr}
	}
	project, err := NewProject(node, columns...)
	if err != nil {
		return nil, err
	}
	node = project

	if query.Limit != nil {
		limit, err := NewLimit(node, *query.Limit)
		if err != nil {
			return nil, err
		}
		node = limit
	}

	plan := &Plan{Root: node}
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	return plan, nil
}

// Validate checks the structural invariants a plan must satisfy before it is
// executed: every input exists, every schema is well formed, and each node's
// configuration matches its schema. It is a programming-error check, not a
// user-facing validator; user mistakes are rejected by internal/validate.
func (p *Plan) Validate() error {
	if p == nil || p.Root == nil {
		return fmt.Errorf("planner: plan has no root")
	}
	return validateNode(p.Root)
}

func validateNode(node Node) error {
	if node == nil {
		return fmt.Errorf("planner: plan node is nil")
	}

	switch n := node.(type) {
	case *Scan:
		if n.Table == nil {
			return fmt.Errorf("planner: scan has no table")
		}
		if n.Table.Schema.Len() == 0 {
			return fmt.Errorf("planner: scan table %q has no columns", n.Table.Name)
		}
		return nil

	case *Filter:
		if err := validateNode(n.Input); err != nil {
			return err
		}
		return validatePredicate(n.Predicate)

	case *Sort:
		if err := validateNode(n.Input); err != nil {
			return err
		}
		if len(n.Terms) == 0 {
			return fmt.Errorf("planner: sort has no terms")
		}
		for _, term := range n.Terms {
			typ, err := eval.Infer(term.Expr)
			if err != nil {
				return fmt.Errorf("planner: sort term: %w", err)
			}
			if typ == catalog.TypeBoolean {
				return fmt.Errorf("planner: sort term is boolean; booleans have no ordering")
			}
		}
		return nil

	case *Project:
		if err := validateNode(n.Input); err != nil {
			return err
		}
		if len(n.Columns) == 0 {
			return fmt.Errorf("planner: projection has no columns")
		}
		if n.Schema().Len() != len(n.Columns) {
			return fmt.Errorf("planner: projection schema has %d columns, want %d", n.Schema().Len(), len(n.Columns))
		}
		for i, column := range n.Columns {
			planned := n.Schema().ColumnAt(i)
			if planned.Name != column.Name || planned.Type != column.Type {
				return fmt.Errorf("planner: projection column %d is %s %s, want %s %s",
					i, planned.Type, planned.Name, column.Type, column.Name)
			}
		}
		return nil

	case *Limit:
		if err := validateNode(n.Input); err != nil {
			return err
		}
		if n.Count < 0 {
			return fmt.Errorf("planner: LIMIT must not be negative, got %d", n.Count)
		}
		return nil

	default:
		return fmt.Errorf("planner: unknown plan node %T", node)
	}
}

func validatePredicate(predicate sqlparse.Expression) error {
	if predicate == nil {
		return fmt.Errorf("planner: filter has no predicate")
	}
	typ, err := eval.Infer(predicate)
	if err != nil {
		return fmt.Errorf("planner: filter predicate: %w", err)
	}
	if typ != catalog.TypeBoolean && typ != catalog.TypeNull {
		return fmt.Errorf("planner: filter predicate is %s, want boolean", typ)
	}
	return nil
}
