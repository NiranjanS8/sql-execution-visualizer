// Package execution implements the pull-based operators that run a validated
// single-table query.
//
// Operators compose: each one asks its input for one row at a time, so scan,
// filter, projection, and limit stream and hold at most one row. Sort is the
// exception: ordering requires every input row, so it materializes its input.
// That cost is bounded by the size of the scanned table, not by the query.
//
// Cancellation flows through context.Context. Every operator checks the context
// before producing a row, and sort also checks while it materializes its input.
// A cancelled operator returns context.Canceled or context.DeadlineExceeded
// unchanged so the caller can classify it.
//
// Operators never mutate rows. Rows returned by scan and filter are the table's
// own slices; callers must treat them as read-only.
package execution

import (
	"context"
	"errors"
	"io"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
)

// Operator is a pull-based row iterator. Next returns io.EOF after the last row,
// and must not be called again after an error other than io.EOF. Schema is fixed
// for the lifetime of the operator.
type Operator interface {
	Schema() catalog.Schema
	Next(ctx context.Context) (catalog.Row, error)
}

// Collect drains op and returns its rows. The result is materialized, so callers
// that need a bound must enforce it before calling or wrap op in a limit.
func Collect(ctx context.Context, op Operator) ([]catalog.Row, error) {
	var rows []catalog.Row
	for {
		row, err := op.Next(ctx)
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
}
