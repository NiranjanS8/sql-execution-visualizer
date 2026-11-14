package execution

import (
	"context"
	"fmt"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/eval"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

// Filter keeps the input rows for which predicate is TRUE. FALSE and NULL both
// drop a row: SQL treats an unknown predicate as "not matched".
type Filter struct {
	input     Operator
	predicate sqlparse.Expression
}

// NewFilter returns an operator that applies predicate to every input row. The
// predicate must have been bound by internal/validate.
func NewFilter(input Operator, predicate sqlparse.Expression) Operator {
	return &Filter{input: input, predicate: predicate}
}

func (f *Filter) Schema() catalog.Schema { return f.input.Schema() }

func (f *Filter) Next(ctx context.Context) (catalog.Row, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		row, err := f.input.Next(ctx)
		if err != nil {
			return nil, err
		}

		keep, err := f.matches(row)
		if err != nil {
			return nil, err
		}
		if keep {
			return row, nil
		}
	}
}

func (f *Filter) matches(row catalog.Row) (bool, error) {
	value, err := eval.Eval(f.predicate, row)
	if err != nil {
		return false, err
	}
	if value.IsNull() {
		return false, nil
	}

	keep, ok := value.AsBool()
	if !ok {
		pos := f.predicate.Position()
		return false, &eval.Error{
			Code:    eval.CodeTypeError,
			Message: fmt.Sprintf("WHERE requires a boolean predicate, found %s.", value.Type()),
			Line:    pos.Line,
			Column:  pos.Column,
		}
	}
	return keep, nil
}
