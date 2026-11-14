package planner

import (
	"fmt"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/execution"
)

// BuildOperators turns a logical plan into the pull-based operator chain that
// runs it. The plan remains the inspectable description; this function is the
// only place that knows how a logical node maps to a physical operator.
func BuildOperators(plan *Plan) (execution.Operator, error) {
	if plan == nil || plan.Root == nil {
		return nil, fmt.Errorf("planner: plan has no root")
	}
	return buildOperator(plan.Root)
}

func buildOperator(node Node) (execution.Operator, error) {
	switch n := node.(type) {
	case *Scan:
		return execution.NewScan(n.Table), nil

	case *Filter:
		input, err := buildOperator(n.Input)
		if err != nil {
			return nil, err
		}
		return execution.NewFilter(input, n.Predicate), nil

	case *Sort:
		input, err := buildOperator(n.Input)
		if err != nil {
			return nil, err
		}
		terms := make([]execution.SortTerm, len(n.Terms))
		for i, term := range n.Terms {
			terms[i] = execution.SortTerm{Expr: term.Expr, Desc: term.Desc}
		}
		return execution.NewSort(input, terms)

	case *Project:
		input, err := buildOperator(n.Input)
		if err != nil {
			return nil, err
		}
		projections := make([]execution.Projection, len(n.Columns))
		for i, column := range n.Columns {
			projections[i] = execution.Projection{
				Name: column.Name,
				Type: column.Type,
				Expr: column.Expr,
			}
		}
		return execution.NewProject(input, projections)

	case *Limit:
		input, err := buildOperator(n.Input)
		if err != nil {
			return nil, err
		}
		return execution.NewLimit(input, n.Count)

	default:
		return nil, fmt.Errorf("planner: unknown plan node %T", node)
	}
}
