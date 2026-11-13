# 0002 — Value and schema model

**Status:** Accepted
**Date:** 2026-11-13
**Phase:** 1

## Context

Parsing, planning, and execution all depend on a shared notion of what a value is, what a column is, and what makes a table valid. Deciding this late would force every later phase to invent its own representation.

The engineering plan requires: typed values rather than stringly-typed data, explicit NULL handling, stable column identity that survives joins, stable row and column ordering, and rejection of inconsistent table definitions.

## Decision

### Four logical types, and NULL as a value

`boolean`, `integer`, `decimal`, `text`. NULL is a *value* type, never a column type — a column cannot be declared NULL.

```go
TypeNull Type = ""        // zero value; the zero Value is NULL
TypeBoolean = "boolean"
TypeInteger = "integer"
TypeDecimal = "decimal"
TypeText    = "text"
```

Making `TypeNull` the zero value means `var v Value` is a usable NULL rather than an invalid sentinel, which removes a class of "did you forget to construct it" bugs.

`NULL` is structurally distinct from `""`, `0`, and `false`. Tests assert this both ways: none of those equal NULL, and NULL does not equal any of them.

### No implicit coercion

`AsBool`, `AsInt`, `AsDecimal`, and `AsText` return `(value, ok)` and set `ok` to false for any other type. Nothing converts an integer to a decimal behind the caller's back. Coercion rules, if any, are decided in Phase 4 alongside expression type checking.

### Two different equalities

`Value.Equals` is *structural*: `NULL` equals `NULL`. It exists for internal use such as grouping keys and test assertions.

SQL comparison is *three-valued* (`NULL = NULL` is unknown) and belongs to expression evaluation in Phase 4. Conflating the two now would make Phase 4 quietly wrong.

### Decimal is `float64` for now — flagged

`decimal` is stored as IEEE-754 binary64. This is a deliberate, temporary simplification: it keeps Phase 1 dependency-free and is adequate for the sample data and most query results.

It is **not** exact decimal arithmetic. `0.1 + 0.2 != 0.3`, and large values lose precision. Phase 4 must decide between keeping binary64 with documented rounding, or moving to exact decimal arithmetic. The decision is tracked in `docs/sql-support.md` and in the known-issues list.

### Nullability is tri-state

```go
NullabilityUnknown  = "unknown"    // not yet known, e.g. before inference
NullabilityNullable = "nullable"
NullabilityNotNull  = "not_null"
```

The plan asks for nullability "if known". A boolean cannot express "unknown", and CSV inference in Phase 2 genuinely needs that state. An empty value normalizes to `unknown`.

`Table.Validate` rejects a NULL in a `not_null` column and allows it otherwise.

### Column identity

Columns carry a `ColumnID`, assigned by `NewSchema` as `c1`, `c2`, … in declaration order. It is unique within a schema and independent of the column name, so identity survives renames and joins.

Duplicate column *names* are permitted in a `Schema` because an inner join of `a.id` and `b.id` legitimately produces two `id` columns that must stay addressable. A base `Table`, however, rejects duplicate names case-insensitively — two columns with the same name in a stored table is a definition error.

### Identifier rules

Table and column names must match `^[A-Za-z_][A-Za-z0-9_]*$`. This is a conservative SQL-ish identifier rule: it keeps names unquoted and unambiguous for the Phase 3 parser. CSV import (Phase 2) is responsible for sanitizing headers and showing the result before commit; it must not silently rename.

### Immutability and ordering

- A `Table` is immutable once registered. The catalog never mutates a table's schema or rows, and callers must not either.
- `Catalog.List` returns registration order, not map order, so results are deterministic.
- `Catalog.List` returns a fresh slice; mutating it cannot affect the catalog.

### Name and ID uniqueness

Table IDs are unique. Table names are unique case-insensitively, because SQL identifiers are case-insensitive. Deleting a table releases both.

## Consequences

- Later phases can rely on `Value` being safe to copy and on NULL being impossible to confuse with a zero value.
- The decimal decision is a real debt with a deadline (Phase 4), not a silent approximation.
- Duplicate-name tolerance in `Schema` means name resolution (Phase 3) must detect ambiguity and report it rather than picking the first match.
- Enforcing identifiers now means Phase 2 must sanitize CSV headers. The import preview already includes a table-name field, so this fits the planned flow.

## Tests

`backend/internal/catalog/*_test.go` covers: NULL distinctness, no-coercion accessors, structural equality, literal rendering, ID assignment and ordering, schema rejection cases, table row validation (width, type, nullability, duplicate names), catalog create/get/list/delete, duplicate ID and name rejection, and the built-in samples.
