package execution

import (
	"context"
	"fmt"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/eval"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

// Projection is one output column of a projection. Type is the static type that
// internal/validate inferred for Expr; TypeNull is allowed for the NULL literal.
type Projection struct {
	Name        string
	Type        catalog.Type
	Nullability catalog.Nullability
	Expr        sqlparse.Expression
}

// Project evaluates one expression per output column for every input row.
type Project struct {
	input       Operator
	projections []Projection
	schema      catalog.Schema
}

// NewProject returns an operator with a result schema built from the projection
// names and types. Unlike a table schema, a result column may have a display name
// that is not an identifier and a type of TypeNull.
func NewProject(input Operator, projections []Projection) (Operator, error) {
	if len(projections) == 0 {
		return nil, fmt.Errorf("execution: projection must declare at least one column")
	}

	defs := make([]catalog.ColumnDef, len(projections))
	for i, projection := range projections {
		defs[i] = catalog.ColumnDef{
			Name:        projection.Name,
			Type:        projection.Type,
			Nullability: projection.Nullability,
		}
	}

	schema, err := catalog.NewResultSchema(defs...)
	if err != nil {
		return nil, err
	}

	return &Project{input: input, projections: projections, schema: schema}, nil
}

func (p *Project) Schema() catalog.Schema { return p.schema }

func (p *Project) Next(ctx context.Context) (catalog.Row, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	row, err := p.input.Next(ctx)
	if err != nil {
		return nil, err
	}

	projected := make(catalog.Row, len(p.projections))
	for i, projection := range p.projections {
		value, err := eval.Eval(projection.Expr, row)
		if err != nil {
			return nil, err
		}
		projected[i] = value
	}
	return projected, nil
}
