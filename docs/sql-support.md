# SQL support

QueryLens implements a deliberately small SQL subset. Anything outside it fails with an explicit error — the engine never parses a construct and silently ignores it.

This table is the contract. It is updated in the same change that implements a feature, never before.

## Status

Phase 3 implements parsing and name resolution. A supported statement is turned into a validated internal structure; **no query is executed yet** — operators and the planner arrive in Phases 5–6. Unsupported syntax is rejected at parse time, before any name is resolved.

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
| Filtering | `WHERE <predicate>` | Rows where the predicate is not `TRUE` will be dropped |
| Ordering | `ORDER BY expr [ASC\|DESC], ...` | Default is ascending |
| Limiting | `LIMIT n` | `n` is a non-negative integer |
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

Errors carry a stable `code` and a 1-based line and column where the parser or validator can supply one:

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

A single-table query cannot produce `ambiguous_column`: table schemas reject duplicate column names. The resolver already handles multiple sources, so the check is in place for Phase 10.
