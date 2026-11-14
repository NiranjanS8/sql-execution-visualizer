# 0004 — Hand-written SQL parser and separate validation

**Status:** Accepted
**Date:** 2026-11-14
**Phase:** 3

## Context

Phase 3 must turn SQL text into stable internal structures, reject unsupported syntax consistently, and resolve names before execution. The engineering plan recommends "a maintained Go SQL parser that exposes an AST" and requires that parser-library types do not leak through the application.

The supported subset is deliberately small: one `SELECT`, one table, a projection, an optional `WHERE`, `ORDER BY`, and `LIMIT`. Everything else — joins, subqueries, CTEs, grouping, set operations, functions — must be refused with a clear error.

## Options considered

| Candidate | Maintained | Dependency weight | Dialect | Problem |
| --- | --- | --- | --- | --- |
| `vitess.io/vitess` (`go/vt/sqlparser`) | Yes | Very large: cloud storage, etcd, Prometheus, gRPC | MySQL | Pulls a server-scale dependency graph into a local single-user tool |
| `github.com/cockroachdb/cockroachdb-parser` | Yes | Large: gRPC, OpenTelemetry, gonum, elastic/gosigar | Postgres | Same problem, different vendor |
| `github.com/pganalyze/pg_query_go` | Yes | cgo + bundled libpg_query | Postgres | Requires a C toolchain; the project is Windows-first |
| `github.com/xwb1989/sqlparser` | No (2018) | Small | MySQL | Unmaintained fork of Vitess |
| `github.com/marianogappa/sqlparser` | No (2021) | Small | MySQL | Incomplete grammar; no maintenance |
| `github.com/blastrain/vitess-sqlparser` | No (2020) | Small | MySQL | Unmaintained |

Every maintained option is either enormous or needs cgo. All of them parse a much larger language than QueryLens supports, so each would still need a rejection pass over the resulting AST — the part that actually enforces the contract.

## Decision

Write a small recursive-descent parser in `internal/sqlparse`, with a separate `internal/validate` for name resolution.

The parser owns:

- lexing (identifiers, quoted identifiers, strings, numbers, operators, comments);
- the grammar for the supported subset;
- classification of recognised-but-unsupported constructs as `unsupported_syntax`, with a specific message;
- `parse_error` for anything the grammar cannot consume.

The validator owns:

- table lookup by name;
- qualifier resolution against the table name or alias;
- column binding, including case-insensitive matching and ambiguity detection;
- wildcard expansion and output naming.

Type checking stays out of Phase 3; expression evaluation and coercion are Phase 4.

## Why a purpose-built parser wins here

- **The rejection layer is the parser.** There is no "parse the whole dialect, then walk the AST looking for things we do not support" gap. A construct either matches the grammar or produces a targeted error at the token that introduced it.
- **Positions are exact.** Every token carries a 1-based line and column, so `GROUP BY` on line 3 is reported at the keyword, not at the end of the statement.
- **No dependency surface.** The backend remains standard-library-only, which matters for a local-first tool that must build on Windows without a C toolchain.
- **The AST is already the domain model.** `internal/sqlparse` emits QueryLens-owned nodes, so the plan's "parser types must not leak" requirement is satisfied by construction rather than by an adapter.

The cost is roughly 700 lines of lexer and parser that the project now owns. That is smaller than the rejection layer a general parser would require, and the grammar is fixed by `docs/sql-support.md`.

## Grammar

```text
statement      := SELECT selectList FROM tableRef
                  [WHERE expr] [ORDER BY orderList] [LIMIT integer] [';'] EOF
selectList     := selectItem (',' selectItem)*
selectItem     := ('*' | qualifier '.' '*') [alias]
                | expr [AS] [alias]
tableRef       := identifier [AS] [alias]
orderList      := expr [ASC | DESC] (',' expr [ASC | DESC])*
expr           := orExpr
orExpr         := andExpr (OR andExpr)*
andExpr        := notExpr (AND notExpr)*
notExpr        := NOT notExpr | comparison
comparison     := additive [('=' | '!=' | '<>' | '<' | '<=' | '>' | '>=') additive]
                | additive IS [NOT] NULL
additive       := multiplicative (('+' | '-') multiplicative)*
multiplicative := unary (('*' | '/') unary)*
unary          := '-' unary | primary
primary        := literal | columnRef | '(' expr ')'
columnRef      := identifier ['.' identifier]
```

Keywords are matched case-insensitively and only when unquoted. `TRUE`, `FALSE`, and `NULL` are literals, not column names.

## Errors

`sqlparse.Error` carries a code, a message, and an optional position. The five codes are stable and shared with the validator: `parse_error`, `unsupported_syntax`, `unknown_table`, `unknown_column`, `ambiguous_column`. The API maps them to the documented envelope in Phase 8.

## Consequences

- The project owns its parser. Grammar changes are local edits plus tests, not version bumps.
- `ORDER BY` currently resolves source columns only. Ordering by an output alias or by position is rejected with an explanatory message; both can be added when the planner decides where sorting sits relative to projection.
- Qualified names are supported for the single table in scope. Phase 10 extends the same resolver to multiple sources; `ambiguous_column` already works and is covered by a direct unit test.
- The parser is strict about statement count: one statement, optional trailing semicolon.
- Leading-dot decimals (`.5`) are not accepted; write `0.5`.
- `SELECT` without `FROM` is refused rather than producing a one-row result, because the initial scope is table queries only.

## Tests

`backend/internal/sqlparse/parser_test.go` covers a full query, star and qualified star, operator precedence and associativity, every literal type, unary minus, `IS NULL`/`IS NOT NULL`, comments, quoted identifiers, case-insensitive keywords, implicit aliases, semicolons, positions, and 45 rejection cases (21 `parse_error`, 24 `unsupported_syntax`).

`backend/internal/validate/validate_test.go` covers projection binding, star expansion order, aliases and implicit aliases, output naming, case-insensitive resolution, qualified references, unknown tables/columns/qualifiers, alias hiding the table name, ambiguity across two sources, unique columns across two sources, `ORDER BY` position and alias rejection, `ORDER BY` expressions, `LIMIT`, and nested `WHERE` resolution.

`backend/internal/catalog/catalog_test.go` covers the new `Lookup` and `Names` methods.
