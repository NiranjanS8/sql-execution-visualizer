# 0006 — Execution operators: pull-based iterators and ordering semantics

**Status:** Accepted
**Date:** 2026-11-14
**Phase:** 5

## Context

Phases 3 and 4 produced a bound, type-checked `validate.Query`: every column reference points at a catalog column, output columns carry inferred types, and expressions can be evaluated against a row. Phase 5 must run that query.

The phase plan asks for an operator contract and execution context, table scan, filter, projection, limit, and ordering, with output schema preserved, cancellation checks, bounded memory, and operator-level tests. It also defers two questions to this phase: where sorting sits relative to projection, and what an empty result means.

## Decision

Execution lives in `internal/execution` and knows nothing about HTTP, JSON, planning, or the frontend. The planner in Phase 6 will build the same operator chain the tests build by hand.

### Operator contract

```go
type Operator interface {
    Schema() catalog.Schema
    Next(ctx context.Context) (catalog.Row, error)
}
```

- Pull-based, one row per call. `io.EOF` means exhausted; any other error ends the operator.
- `Schema` is fixed for the operator's lifetime.
- Cancellation travels through `context.Context`. Operators check it before producing a row; a cancelled operator returns `context.Canceled` or `context.DeadlineExceeded` unchanged so Phase 8 can map it to `timeout`.
- Operators never mutate rows. Scan and filter return the table's own slices, documented as read-only.

Pull was chosen over push because composition, per-operator instrumentation (Phase 7), and cancellation are easier to reason about when each operator controls when it asks for the next row.

### Operators

| Operator | Behaviour | Memory |
| --- | --- | --- |
| `Scan` | Emits a table's rows in stored order | Constant |
| `Filter` | Evaluates `WHERE`; keeps only `TRUE` (FALSE and NULL drop the row) | Constant |
| `Project` | Evaluates one expression per output column, building a fresh row | Constant |
| `Limit` | Emits at most `n` rows; `LIMIT 0` never reads its input | Constant |
| `Sort` | Materializes its input, computes one key per term per row, stable-sorts | O(rows × terms) |

`Collect` drains an operator into a slice for callers and tests. It materializes by definition; result-row limits are Phase 8's responsibility.

### Pipeline order

`Scan → Filter → Sort → Project → Limit`.

`ORDER BY` currently resolves source columns only, so sorting must see source rows, not projected rows. Projection then builds the result schema, and limit applies to the projected result. When `ORDER BY` later accepts output aliases or positions, the planner will have to split projection or sort on computed keys; that is a Phase 6+ decision, not a reason to reorder now.

### Ordering semantics

- **Stable.** `sort.SliceStable` on rows plus precomputed keys. Rows with equal keys keep input order. The phase plan requires stability to be promised and tested; the tests assert it.
- **NULLs last in both directions.** A NULL key sorts after every known key, in `ASC` and `DESC`. `DESC` reverses the comparison of known values only. This avoids the common surprise where `ORDER BY x DESC` puts missing data first.
- **Booleans rejected.** `validate.Resolve` now returns `type_error` for a boolean `ORDER BY`, and `NewSort` repeats the check so the operator is safe when tested alone. Inventing `FALSE < TRUE` would add semantics the subset does not need.
- **Keys are computed once.** Evaluating keys while materializing makes errors deterministic and avoids re-evaluating expressions during comparisons. Memory is bounded by the scanned table, which CSV import already caps.

### Projection schema

`catalog.NewResultSchema` assigns `c1..cN` and validates a *result* schema: a display name may be any non-empty text, and `TypeNull` is allowed for the `NULL` literal. Table schemas keep the stricter identifier rule and reject `TypeNull`.

`Projection` carries the name, type, and expression. Nullability defaults to `unknown`; a later phase can propagate it when plan schemas are finalized.

### Empty results

A result is a schema plus zero or more rows. There is no separate "empty" state: an empty result is a zero-row result with a known schema. This resolves the open question carried since Phase 2 and keeps the API shape simple.

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Push-based operators | Harder to compose and instrument; cancellation becomes everyone's problem at once |
| `Next() (row, ok)` without errors | Hides evaluation and cancellation failures, or forces a separate error channel |
| Sort before filter | Same rows, more work; filter should shrink the sort input |
| NULLs first in `DESC` | Matches some engines but surprises users and makes placement direction-dependent |
| Allow boolean ordering as `FALSE < TRUE` | Invents semantics; `=` and `!=` already cover booleans |
| Stream sorting with a bounded heap | Only valid when a limit is known and changes stability guarantees; premature for a table-sized input |
| Copy every row at every operator | Unnecessary allocation; rows are immutable by convention |

## Consequences

- The full single-table pipeline runs in tests without HTTP or a planner.
- Sorting is the only materializing operator; its memory is bounded by the table, which is already capped at 100,000 rows by CSV import.
- `LIMIT 0` short-circuits without touching its input, which matters once scan sits over an expensive operator.
- Cancellation is observable and testable per operator.
- Boolean `ORDER BY` moved from "accepted by validation, semantics pending" to a clear `type_error`.
- Phase 6 can build the operator tree from `validate.Query` without new semantics: the mapping is one operator per clause, in the order above.

## Tests

`backend/internal/execution/execution_test.go` covers:

- scan order, schema, repeated `io.EOF`, and cancellation;
- filter keeps only TRUE, drops NULL, matches `IS NULL`, rejects a non-boolean predicate, and preserves the schema;
- projection schema, values, `TypeNull` result columns, empty projections, and evaluation errors;
- limit count, `LIMIT 0` not reading its input, a limit above the input size, and negative counts;
- sort ascending/descending with NULLs last, stability, multiple terms, boolean rejection, no terms, cancellation, and key-evaluation errors.

`backend/internal/execution/pipeline_test.go` parses and validates a real query, assembles `Scan → Filter → Sort → Project → Limit`, and asserts the ordered result and its schema.

`backend/internal/eval/eval_test.go` adds direct `Compare` tests: numeric widening, text byte order, NULL rejection, boolean rejection, and mismatched types.

`backend/internal/validate/validate_test.go` adds the boolean `ORDER BY` rejection.
