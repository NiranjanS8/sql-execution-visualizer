# 0007 — Logical planner: plan tree, operator order, and explain output

**Status:** Accepted
**Date:** 2026-11-14
**Phase:** 6

## Context

Phase 5 produced pull-based operators, but only tests could assemble them. Phase 6 must convert a validated `validate.Query` into a logical plan, define operator ordering, place projection without changing semantics, validate plan output schemas, provide an inspectable form, and prove the whole path from SQL text to result rows.

The implementation plan also asks that logical planning stay separate from physical execution, and that visualization DTOs stay separate from internal operator structs.

## Decision

Planning lives in `internal/planner`. `planner.Build(query)` returns a `*Plan`, a tree of logical nodes. `planner.BuildOperators(plan)` maps that tree to the `execution.Operator` chain. The plan is data and remains independent of JSON, React Flow, and HTTP.

### Plan tree

```go
type Node interface {
    Kind() Kind
    Children() []Node
    Schema() catalog.Schema
    Describe() string
}
```

Five node types mirror the supported clauses: `Scan`, `Filter`, `Sort`, `Project`, and `Limit`. Each node exposes:

- its kind, for tests and graph rendering;
- its children, in data-flow order (all nodes have zero or one child today);
- its output schema, so the plan can be validated without executing it;
- a one-line description, for explain output.

Plan nodes keep the expressions from `validate.Query` unchanged. The planner does not re-parse, re-bind, or re-infer types; validation already did that work.

### Operator order

`Scan → Filter → Sort → Project → Limit`.

- **Filter below sort.** `WHERE` shrinks the input before the sort materializes it.
- **Sort below project.** `ORDER BY` resolves source columns only, so it must see source rows. Sorting after projection would require projected rows to retain every source column, or would fail for `ORDER BY salary` when `salary` is not selected.
- **Projection builds the result schema.** The root schema is the projection's schema unless `LIMIT` wraps it, in which case the limit preserves it.
- **Limit above project.** `LIMIT` applies to result rows. Row count is the same either way today, but the placement matches SQL semantics and leaves room for later `DISTINCT` or aggregation.

This is also the order the Phase 5 tests hand-built, so the planner does not introduce new execution semantics.

### Projection and aliases

`Project` columns carry the output name, inferred type, and expression from `validate.OutputColumn`. Aliases therefore become result column names, and a plain column reference without an alias keeps the source name. `SELECT name AS employee` produces a column named `employee`; `ORDER BY salary` still sorts on the source column because sorting happens below projection.

`Project` builds its schema with `catalog.NewResultSchema`, so display names need not be identifiers and `SELECT NULL` has type `TypeNull`.

### Schema validation

`Plan.Validate` walks the tree and checks:

- every input exists and the root is present;
- scan tables are non-nil and have a schema;
- filter predicates are boolean (or the NULL literal);
- sort terms are present and not boolean;
- projection columns are non-empty and match the project schema position by position;
- limit counts are non-negative;
- node types are known.

`Build` calls `Validate` before returning, so an invalid plan cannot leave the package. These are programming-error checks: user mistakes are still rejected by `internal/validate` with stable error codes.

### Explain output

`Plan.Explain()` renders an indented tree, one node per line:

```
limit 2
  project name AS employee, (salary * 2) AS doubled
    sort salary DESC
      filter (salary > 120000)
        scan employees
```

Expressions are rendered by a small debug formatter in `internal/planner`. Binary expressions are fully parenthesized, so precedence cannot be misread. It is not a SQL round-trip and does not aim to be one. The formatter covers every expression node the parser can produce: literals, columns, qualified names, stars, unary operators, all binary operators, and `IS [NOT] NULL`.

This satisfies "inspectable logical plan" without committing to the graph DTO shape, which belongs to Phase 7 (IDs, metrics) and Phase 9 (visualization).

### Operator mapping

`BuildOperators` is the only function that knows how a logical node becomes a physical operator. It maps planner `SortTerm` and `Column` values to their `execution` equivalents. Keeping this mapping in the planner means `internal/execution` stays a generic operator package that knows nothing about planning, and there is no import cycle.

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Planner returns only an `execution.Operator` | No inspectable plan for tests, Phase 7, or the graph |
| Plan nodes are the execution operators | Couples logical description to physical behaviour and blocks instrumentation |
| Generic `{kind, config map}` nodes | Loses type safety; every consumer has to type-assert maps |
| Project before sort | Breaks `ORDER BY` on columns that are not selected |
| Sort after limit | Changes which rows are returned |
| JSON plan DTO now | The graph shape belongs to Phases 7 and 9; a premature DTO would be rewritten |
| Re-infer types in the planner | Duplicates `eval.Infer`; validation is the single source of type truth |

## Consequences

- The full path SQL → parse → validate → plan → operators → rows is covered by tests.
- Plans can be inspected and asserted without executing them.
- Phase 7 can attach IDs, timings, and row counts to plan nodes without changing operator semantics.
- Phase 8 can expose `Plan.Explain()` (or a future DTO) as debug information while the HTTP response shape is decided.
- `internal/planner` imports `internal/execution`; the dependency points from planning to execution, never back.

## Tests

`backend/internal/planner/planner_test.go` covers:

- operator order for projection-only, filter, sort, limit, and all-clause queries;
- output names and types for aliases, expressions, and `NULL`;
- exact explain output, including qualified source columns;
- the debug formatter for every supported expression node and operator;
- end-to-end execution of wildcard projection, filtering, NULL handling, aliases, qualified names, ascending/descending sort, `LIMIT 0`, and empty results;
- schema preservation through filter, sort, and limit;
- construction and validation failures: nil query, missing source table, nil scan, missing or non-boolean filter, missing or boolean sort terms, empty projection, projection schema mismatch, negative limit, unknown node, and nil plans.
