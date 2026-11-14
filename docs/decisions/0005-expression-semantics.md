# 0005 — Expression semantics: three-valued logic, coercion, and errors

**Status:** Accepted
**Date:** 2026-11-14
**Phase:** 4

## Context

Phase 3 produced a bound `validate.Query`: every column reference points at a catalog column, and output names are known. Phase 4 must evaluate those expressions and decide the questions Phase 1 deferred:

- how `NULL` propagates through arithmetic, comparison, and boolean operators;
- which operand types are compatible, and whether coercion is implicit;
- what happens on integer versus decimal division, division by zero, and overflow;
- how text compares;
- whether `decimal` becomes exact arithmetic.

The engine never approximates unsupported behaviour, so each answer has to be explicit and testable.

## Decision

Expression semantics live in `internal/eval`. `validate.Resolve` applies `eval.Infer` to the projection, `WHERE`, and `ORDER BY`, so execution only ever sees a type-checked query.

### Static types

`eval.Infer` returns one of `boolean`, `integer`, `decimal`, `text`, or `TypeNull` ("unknown"). A `NULL` literal is compatible with any operand; the operation itself yields `NULL`.

`validate.OutputColumn` now carries the inferred `Type`, so later phases do not have to re-infer result schemas.

### Coercion

There is exactly one widening rule: integer and decimal may be combined.

- Arithmetic with two integers returns an integer; any decimal operand makes the result decimal.
- Comparison of an integer with a decimal converts the integer to binary64.
- Text, boolean, and numeric types never mix. `name + 1`, `name = 1`, and `NOT 1` are `type_error`, not guesses.

This preserves the Phase 1 rule that values do not silently change type, while keeping the common `salary > 100` case working.

### Three-valued logic

`NULL` is unknown, not false.

- Arithmetic and comparison with `NULL` produce `NULL`.
- `NOT NULL` is `NULL`.
- `AND` and `OR` follow the SQL truth tables. `FALSE AND NULL` is `FALSE`; `TRUE OR NULL` is `TRUE`; `NULL AND TRUE` is `NULL`; `NULL OR FALSE` is `NULL`.
- `IS NULL` and `IS NOT NULL` are the only operators that always return `TRUE` or `FALSE`.
- `AND` and `OR` evaluate left to right and short-circuit when the left operand decides the result. This makes error visibility deterministic: `FALSE AND 1/0 = 1` is `FALSE`, while `TRUE AND 1/0 = 1` raises `execution_error`.

`WHERE` requires a boolean predicate. A `NULL` predicate is allowed and matches no rows; an integer or text predicate is a `type_error` at validation time.

### Arithmetic

- `+`, `-`, `*`, `/`, and unary `-` require numeric operands.
- Integer division truncates toward zero, matching Go and common SQL engines: `7 / 2 = 3`, `-7 / 2 = -3`.
- Division by zero raises `execution_error`. It never returns `NULL` or Infinity.
- Integer overflow raises `execution_error`; it never wraps. Checked cases include addition, subtraction, multiplication, division (`MinInt64 / -1`), and negation (`-MinInt64`).
- A decimal result that is infinite or NaN raises `execution_error`.

### Comparison

- `=` and `!=` work for every type, with numeric widening.
- `<`, `<=`, `>`, `>=` work for integer, decimal, and text. Booleans support only `=` and `!=`; there is no invented `FALSE < TRUE` ordering.
- Text compares byte-wise and case-sensitive: `'B' < 'a'` is true.

### Decimal representation

`decimal` stays IEEE-754 binary64. Exact decimal arithmetic is **not** implemented.

Reasons:

- Changing `catalog.Value` to an exact representation would rewrite the Phase 1 value model, CSV inference, JSON serialization, and every arithmetic test.
- The initial scope in the engineering plan explicitly accepts binary64 as debt; the goal of Phase 4 is to make the behaviour explicit, not to redesign the value model.
- A learning tool that documents the limitation is honest; one that silently claims exactness is not.

Consequences, stated in `docs/sql-support.md`: `0.1 + 0.2` is not exactly `0.3`, and integers beyond 2^53 lose precision when mixed with decimal. Exact decimal is a candidate for a dedicated later phase.

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Coerce everything to text or decimal | Hides mistakes and contradicts "no implicit coercion" |
| Return `NULL` on division by zero | Makes an error look like missing data |
| Saturate on integer overflow | Silently changes the answer |
| Allow boolean ordering | Invents semantics the subset does not need |
| Exact decimal now (`math/big.Rat`) | Large cross-cutting change outside the phase; better as its own decision |

## Consequences

- Invalid operand types are caught by `validate.Resolve`, before any operator runs.
- Output columns have types, which Phase 6 planning and Phase 8 result rendering can use directly.
- Two error types cross the pipeline: `sqlparse.Error` for syntax/name failures and `eval.Error` for type/evaluation failures. Phase 8 maps both to the HTTP envelope; the stable codes are shared in `docs/sql-support.md`.
- Evaluation errors carry the source position of the expression node.
- `ORDER BY` type validity is checked, but sort ordering itself is Phase 5. Booleans are currently accepted in `ORDER BY` and will be rejected or ordered when the sort operator is defined.

## Tests

`backend/internal/eval/eval_test.go` covers:

- inferred types for literals, columns, arithmetic, comparison, boolean operators, and `IS NULL`;
- every rejection for incompatible operands, including text arithmetic, boolean ordering, and non-boolean `WHERE`;
- integer and decimal arithmetic, mixed numeric arithmetic, truncation toward zero, and NULL propagation;
- division by zero, every integer overflow path, decimal overflow, and negation overflow;
- all 9 `AND` and 9 `OR` three-valued combinations plus the three `NOT` cases;
- short-circuit behaviour for `AND` and `OR`;
- `IS NULL` / `IS NOT NULL`, source positions, unbound columns, short rows, and wildcards.

`backend/internal/validate/validate_test.go` adds output-type assertions and rejects invalid expression types and non-boolean predicates.
