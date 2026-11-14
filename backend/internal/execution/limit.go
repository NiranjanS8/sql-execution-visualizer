package execution

import (
	"context"
	"fmt"
	"io"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
)

// Limit emits at most count input rows. LIMIT 0 emits nothing and never pulls
// its input.
type Limit struct {
	input   Operator
	count   int
	emitted int
}

// NewLimit returns an operator that stops after count rows. A negative count is a
// programming error; the parser rejects it before planning.
func NewLimit(input Operator, count int) (Operator, error) {
	if count < 0 {
		return nil, fmt.Errorf("execution: LIMIT must not be negative, got %d", count)
	}
	return &Limit{input: input, count: count}, nil
}

func (l *Limit) Schema() catalog.Schema { return l.input.Schema() }

func (l *Limit) Next(ctx context.Context) (catalog.Row, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if l.emitted >= l.count {
		return nil, io.EOF
	}

	row, err := l.input.Next(ctx)
	if err != nil {
		return nil, err
	}
	l.emitted++
	return row, nil
}
