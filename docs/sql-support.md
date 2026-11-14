# SQL support

QueryLens implements a deliberately small SQL subset. Anything outside it fails with an explicit error — the engine never parses a construct and silently ignores it.

This table is the contract. It is updated in the same change that implements a feature, never before.

## Status

Phase 5 implements parsing, name resolution, static type checking, expression evaluation, and the pull-based operators that run a single-table query: scan, filter, sort, project, and limit. A supported statement is turned into a validated structure whose output columns carry inferred types, and the operators can execute it. **No HTTP query endpoint exists yet** — the planner and query API arrive in Phases 6 and 8. Unsupported syntax is rejected at parse time, before any name is resolved.

## Supported now

| Feature | Syntax | Notes |
| --- | --- | --- |
| Projection | `SELECT a, b`, `SELECT *`, `SELECT t.*` | `*` expands in schema order |
| Aliases | `SELECT a AS x`, `SELECT a x`, `FROM t AS u` | Output aliases are optional; duplicate output names are allowed |
| Source | `FROM one_table` | Single table only until Phase 10 |
| Qualified names | `t.col` | One qualifier; must be the table name or its alias |
| Literals | `'text'`, `42`, `3.5`, `TRUE`, `FALSE`, `NULL` | Single quotes escape as `''` |
| Comparison | `=`, `!=`, `<>`, `<`, `<=`, `>`, `>=` | `NULL` compares to nothing; use `IS NULL` |
| Boolean | `AND`, `OR`, `NOT` | `NOT` binds tighter than `AND`, which binds tighter than `OR` |
| Arithmetic | `+`, `-`, `*`, `/`, unary `-` | `*` and `/` bind tighter than `+` and `-` |
| Filtering | `WHERE <predicate>` | Only rows where the predicate is `TRUE` are kept |
| Ordering | `ORDER BY expr [ASC\|DESC], ...` | Stable; NULLs sort last in both directions; booleans are rejected |
| Limiting | `LIMIT n` | `n` is a non-negative integer; `LIMIT 0` returns no rows |
| Null tests | `IS NULL`, `IS NOT NULL` | The only correct way to test for null |
| Comments | `-- line`, `/* block */` | Ignored by the lexer |
| Identifiers | `name`, `"name"` | Double quotes allow reserved words and spaces; matching is case-insensitive |
| Statements | one `SELECT`, optional trailing `;` | Multiple statements are rejected |

### Output names

A result column is named, in order of preference:

1. the alias, when one is written;
2. the source column's canonical name, for a plain column reference;
3. `expr_N`, where `N` is the 1-based position in the projection.

Two columns may share a name. They stay distinguishable by position; the API returns columns in order.

### Identifier rules

Identifiers follow the catalog rule: letters, digits, and underscores, starting with a letter or underscore. Bare identifiers are matched case-insensitively, so `Name`, `name`, and `NAME` all resolve to the same column. A reserved word used as an identifier must be double-quoted, for example `SELECT "order" FROM "from"`.

## Explicitly unsupported

These are rejected with `unsupported_syntax`, not approximated:

- Subqueries and correlated subqueries
- Common table expressions (`WITH`)
- Window functions
- `GROUP BY`, `HAVING`, and aggregate functions
- `DISTINCT`
- `UNION`, `INTERSECT`, `EXCEPT`
- Joins (until Phase 10)
- `IN`, `BETWEEN`, `LIKE`, `ILIKE`
- `CASE` expressions and `CAST`
- `OFFSET`, `FETCH`, `LIMIT n, m`, and `NULLS FIRST`/`NULLS LAST`
- `ORDER BY` by output position (`ORDER BY 1`) and by output alias
- `INSERT`, `UPDATE`, `DELETE`, `CREATE`, `DROP`, `ALTER`, `EXPLAIN`, `PRAGMA`
- Vendor-specific extensions (`::` casts, backtick identifiers)
- Unary `+`
- Implicit type coercion beyond the documented rules

## Semantics

Decided in [ADR 0002](decisions/0002-value-and-schema-model.md), [ADR 0005](decisions/0005-expression-semantics.md), and [ADR 0006](decisions/0006-execution-operators.md), and covered by tests.

### Types and coercion

- Four logical types: `boolean`, `integer`, `decimal`, `text`. `NULL` is a value, never a column type.
- `NULL` is distinct from `''`, `0`, and `false`, and never coerces to them.
- There is no implicit coercion between text, boolean, and numeric types. `name + 1` is a `type_error`, not a guess.
- The one widening rule: integer and decimal may be combined. Arithmetic returns `integer` when both operands are integer, otherwise `decimal`. Comparison converts the integer to binary64.
- `decimal` is IEEE-754 binary64, **not** exact decimal arithmetic. `0.1 + 0.2` is not exactly `0.3`, and integers beyond 2^53 lose precision when mixed with decimal. This is accepted debt, documented rather than hidden.

### NULL and three-valued logic

- Arithmetic and comparison with `NULL` produce `NULL`.
- `NOT NULL` is `NULL`.
- `AND` and `OR` follow the SQL truth tables: `FALSE AND NULL` is `FALSE`; `TRUE OR NULL` is `TRUE`; `NULL AND TRUE` is `NULL`; `NULL OR FALSE` is `NULL`.
- `IS NULL` and `IS NOT NULL` always return `TRUE` or `FALSE`.
- `AND` and `OR` evaluate left to right and short-circuit when the left operand decides the result, so a right-hand error is not raised in that case.

### Arithmetic

- `+`, `-`, `*`, `/`, and unary `-` require numeric operands.
- Integer division truncates toward zero: `7 / 2 = 3`, `-7 / 2 = -3`.
- Division by zero is an `execution_error`, never `NULL` or Infinity.
- Integer overflow is an `execution_error`; it never wraps. This includes negating the minimum int64 and `MinInt64 / -1`.
- A decimal result that is infinite or NaN is an `execution_error`.

### Comparison and ordering

- `=` and `!=` work for all types, with the numeric widening rule above.
- `<`, `<=`, `>`, and `>=` work for integer, decimal, and text. Booleans have no ordering; use `=` or `!=`.
- Text comparison is byte-wise and case-sensitive: `'B' < 'a'` is true.
- `WHERE` must be boolean. A `NULL` predicate is allowed and matches no rows. Output columns carry their inferred type.

### Ordering and limiting

- `ORDER BY` evaluates each term against the source row, so it can only reference source columns, not output aliases or positions.
- Sorting is stable: rows that compare equal keep their input order.
- `NULL` sorts after every non-NULL value in both `ASC` and `DESC`. `DESC` reverses the order of known values only, never the placement of NULLs.
- Boolean expressions are rejected in `ORDER BY` with `type_error`; booleans have no ordering.
- Sorting materializes its input. The cost is bounded by the scanned table size, not by the query.
- `LIMIT 0` returns no rows and never reads its input. A limit larger than the input returns every row. `LIMIT` is applied after projection.

Open questions:

| Question | Status |
| --- | --- |
| Whether an empty result is distinguishable from a zero-row result | Resolved in Phase 5 — a result is its schema plus zero or more rows; there is no separate empty state |
| Whether `decimal` becomes exact arithmetic | Deferred; binary64 is accepted debt (ADR 0005) |

## Reading errors

Errors carry a stable `code` and a 1-based line and column where the parser, validator, or evaluator can supply one:

```json
{
  "error": {
    "code": "unsupported_syntax",
    "message": "Window functions are not supported.",
    "location": { "line": 2, "column": 8 }
  }
}
```

| Code | Raised by | Meaning |
| --- | --- | --- |
| `parse_error` | `internal/sqlparse` | The text is not valid SQL in the supported grammar. |
| `unsupported_syntax` | `internal/sqlparse`, `internal/validate` | Valid SQL, but outside the subset. |
| `unknown_table` | `internal/validate` | The `FROM` table, or a qualifier, does not resolve. |
| `unknown_column` | `internal/validate` | A column reference does not resolve. |
| `ambiguous_column` | `internal/validate` | A column reference matches more than one source. |
| `type_error` | `internal/eval`, `internal/validate`, `internal/execution` | An operator cannot accept its operand types, `WHERE` is not boolean, or `ORDER BY` is boolean. |
| `execution_error` | `internal/eval` | Evaluation failed: division by zero, numeric overflow, or an unbound column. |
| `timeout` | Phase 8 | The request exceeded its deadline. Operators return `context.Canceled`/`context.DeadlineExceeded` unchanged so the API can classify it. |

A single-table query cannot produce `ambiguous_column`: table schemas reject duplicate column names. The resolver already handles multiple sources, so the check is in place for Phase 10.
