package execution

import (
	"context"
	"errors"
	"io"
	"sort"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/catalog"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/eval"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/sqlparse"
)

// SortTerm is one ORDER BY term. Expressions are evaluated against input rows,
// which is why the sort operator sits before projection.
type SortTerm struct {
	Expr sqlparse.Expression
	Desc bool
}

type sortEntry struct {
	row  catalog.Row
	keys []catalog.Value
}

// Sort materializes its input and orders it stably. Rows that compare equal keep
// their input order, and NULLs sort last in both directions.
type Sort struct {
	input   Operator
	terms   []SortTerm
	entries []sortEntry
	next    int
	loaded  bool
	loadErr error
}

// NewSort returns an operator that orders input by terms. With no terms it
// returns input unchanged, because ORDER BY with nothing to order by cannot
// reorder rows.
func NewSort(input Operator, terms []SortTerm) (Operator, error) {
	if len(terms) == 0 {
		return input, nil
	}

	for _, term := range terms {
		typ, err := eval.Infer(term.Expr)
		if err != nil {
			return nil, err
		}
		if typ == catalog.TypeBoolean {
			pos := term.Expr.Position()
			return nil, &eval.Error{
				Code:    eval.CodeTypeError,
				Message: "ORDER BY cannot sort boolean expressions; booleans have no ordering.",
				Line:    pos.Line,
				Column:  pos.Column,
			}
		}
	}

	return &Sort{input: input, terms: terms}, nil
}

func (s *Sort) Schema() catalog.Schema { return s.input.Schema() }

func (s *Sort) Next(ctx context.Context) (catalog.Row, error) {
	if !s.loaded {
		s.loaded = true
		s.loadErr = s.load(ctx)
	}
	if s.loadErr != nil {
		return nil, s.loadErr
	}

	if s.next >= len(s.entries) {
		return nil, io.EOF
	}

	row := s.entries[s.next].row
	s.next++
	return row, nil
}

func (s *Sort) load(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		row, err := s.input.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		keys := make([]catalog.Value, len(s.terms))
		for i, term := range s.terms {
			value, err := eval.Eval(term.Expr, row)
			if err != nil {
				return err
			}
			keys[i] = value
		}
		s.entries = append(s.entries, sortEntry{row: row, keys: keys})
	}

	var compareErr error
	sort.SliceStable(s.entries, func(i, j int) bool {
		if compareErr != nil {
			return false
		}
		order, err := compareKeys(s.entries[i].keys, s.entries[j].keys, s.terms)
		if err != nil {
			compareErr = err
			return false
		}
		return order < 0
	})
	return compareErr
}

// compareKeys returns -1, 0, or 1 in the requested direction. NULLs are ordered
// after every non-NULL value regardless of direction, so DESC reverses values
// but not the placement of unknown keys.
func compareKeys(left, right []catalog.Value, terms []SortTerm) (int, error) {
	for i, term := range terms {
		switch {
		case left[i].IsNull() && right[i].IsNull():
			continue
		case left[i].IsNull():
			return 1, nil
		case right[i].IsNull():
			return -1, nil
		}

		order, err := eval.Compare(left[i], right[i])
		if err != nil {
			return 0, err
		}
		if order == 0 {
			continue
		}
		if term.Desc {
			order = -order
		}
		return order, nil
	}
	return 0, nil
}
