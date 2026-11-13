# SQL support

QueryLens implements a deliberately small SQL subset. Anything outside it fails with an explicit error — the engine never parses a construct and silently ignores it.

This table is the contract. It is updated in the same change that implements a feature, never before.

## Supported now

Nothing yet. The parser lands in Phase 3.

## Initial scope (Phases 3–6)

| Feature | Syntax | Notes |
| --- | --- | --- |
| Projection | `SELECT a, b`, `SELECT *` | `*` expands in schema order |
| Aliases | `SELECT a AS x`, `FROM t AS u` | Duplicate output names are allowed but must be addressable |
| Source | `FROM one_table` | Single table only until Phase 10 |
| Literals | `'text'`, `42`, `3.14`, `TRUE`, `FALSE`, `NULL` | No implicit casts between types |
| Comparison | `=`, `!=`, `<>`, `<`, `<=`, `>`, `>=` | `NULL` compares to nothing; use `IS NULL` |
| Boolean | `AND`, `OR`, `NOT` | SQL three-valued logic, not two-valued |
| Filtering | `WHERE <predicate>` | Rows where the predicate is not `TRUE` are dropped |
| Ordering | `ORDER BY col [ASC\|DESC], ...` | Stable sort; ties keep input order |
| Limiting | `LIMIT n` | Applied after ordering |
| Arithmetic | `+`, `-`, `*`, `/` | Division by zero is an error, not `NULL` or `Infinity` |
| Null tests | `IS NULL`, `IS NOT NULL` | The only correct way to test for null |

## Subsequent scope

| Feature | Phase |
| --- | --- |
| Multiple tables, qualified and unqualified column names | 10 |
| `INNER JOIN`, `LEFT JOIN` | 10 |
| `GROUP BY`, `HAVING` | 11 |
| `COUNT`, `SUM`, `AVG`, `MIN`, `MAX` | 11 |
| `DISTINCT` | 11 |
| `UNION ALL`, then `UNION` | later |
| Deliberately implemented scalar functions | later |

## Explicitly unsupported

These are rejected with `unsupported_syntax`, not approximated:

- Subqueries and correlated subqueries
- Common table expressions (`WITH`)
- Window functions
- `INSERT`, `UPDATE`, `DELETE`, `CREATE`, `DROP`, `ALTER`
- Transactions
- Indexes and storage management
- Vendor-specific extensions (`::` casts, `LIMIT n, m`, `ILIKE`, backtick identifiers)
- Implicit type coercion beyond the documented rules

## Semantics

Decided in [ADR 0002](decisions/0002-value-and-schema-model.md) and covered by tests:

- Four logical types: `boolean`, `integer`, `decimal`, `text`. `NULL` is a value, never a column type.
- `NULL` is distinct from `''`, `0`, and `false`, and never coerces to them.
- Values are copied, never aliased. No implicit coercion between types.
- `decimal` is currently IEEE-754 binary64 — **not** exact decimal arithmetic.

Still to be decided before the feature ships:

| Question | Decided in |
| --- | --- |
| `NULL` propagation through arithmetic, comparison, and boolean operators | Phase 4 |
| Numeric overflow policy (error versus saturation) | Phase 4 |
| Integer versus decimal division, and division by zero | Phase 4 |
| String comparison rules (byte order, case sensitivity) | Phase 4 |
| Whether `decimal` becomes exact arithmetic | Phase 4 |
| Whether an empty result is distinguishable from a zero-row result | Phase 5 |

## Reading errors

Errors carry a stable `code` and, where the parser can supply one, a line and column:

```json
{
  "error": {
    "code": "unsupported_syntax",
    "message": "Window functions are not supported.",
    "location": { "line": 2, "column": 8 }
  }
}
```
