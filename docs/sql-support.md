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

## Semantics to be fixed before implementation

Each of these is decided and written down before the feature ships, then tested:

- Supported logical types and their Go representations
- `NULL` propagation through arithmetic, comparison, and boolean operators
- Numeric overflow policy (error versus saturation)
- Integer versus decimal division
- String comparison rules (byte order, case sensitivity)
- Whether and when an empty result is distinguishable from a zero-row result

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
