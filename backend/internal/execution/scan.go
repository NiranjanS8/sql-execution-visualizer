package execution

import (
	"context"
	"io"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
)

// Scan emits a table's rows in stored order.
type Scan struct {
	table *catalog.Table
	next  int
}

// NewScan returns an operator over table. The table must not be nil.
func NewScan(table *catalog.Table) Operator {
	return &Scan{table: table}
}

func (s *Scan) Schema() catalog.Schema { return s.table.Schema }

func (s *Scan) Next(ctx context.Context) (catalog.Row, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.next >= len(s.table.Rows) {
		return nil, io.EOF
	}

	row := s.table.Rows[s.next]
	s.next++
	return row, nil
}
