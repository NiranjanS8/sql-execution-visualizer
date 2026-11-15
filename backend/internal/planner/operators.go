package planner

import (
	"fmt"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/execution"
)

// BuildOperators turns a logical plan into the pull-based operator chain that
// runs it. The plan remains the inspectable description; BuildOperator owns the
// per-node logical-to-physical mapping.
func BuildOperators(plan *Plan) (execution.Operator, error) {
	if plan == nil || plan.Root == nil {
		return nil, fmt.Errorf("planner: plan has no root")
	}
	return buildOperatorTree(plan.Root)
}

func buildOperatorTree(node Node) (execution.Operator, error) {
	children := node.Children()
	if len(children) > 1 {
		return nil, fmt.Errorf("planner: node %s has %d inputs; only one is supported", node.Kind(), len(children))
	}

	var input execution.Operator
	if len(children) == 1 {
		child, err := buildOperatorTree(children[0])
		if err != nil {
			return nil, err
		}
		input = child
	}

	return BuildOperator(node, input)
}

// BuildOperator maps one logical node to its physical operator. input is the
// already-built child operator; Scan ignores it, and every other node requires
// it. Instrumentation calls this function per node so the logical-to-physical
// mapping lives in exactly one place.
func BuildOperator(node Node, input execution.Operator) (execution.Operator, error) {
	switch n := node.(type) {
	case *Scan:
		return execution.NewScan(n.Table), nil

	case *Filter:
		if input == nil {
			return nil, fmt.Errorf("planner: filter has no input")
		}
		return execution.NewFilter(input, n.Predicate), nil

	case *Sort:
		if input == nil {
			return nil, fmt.Errorf("planner: sort has no input")
		}
		terms := make([]execution.SortTerm, len(n.Terms))
		for i, term := range n.Terms {
			terms[i] = execution.SortTerm{Expr: term.Expr, Desc: term.Desc}
		}
		return execution.NewSort(input, terms)

	case *Project:
		if input == nil {
			return nil, fmt.Errorf("planner: project has no input")
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
		if input == nil {
			return nil, fmt.Errorf("planner: limit has no input")
		}
		return execution.NewLimit(input, n.Count)

	default:
		return nil, fmt.Errorf("planner: unknown plan node %T", node)
	}
}
