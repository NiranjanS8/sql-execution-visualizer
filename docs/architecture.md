# Architecture

QueryLens is a local-first application with a Go API and a React frontend. The backend owns all execution truth: it parses, validates, plans, and executes queries, and returns the resulting rows together with a graph of what happened. The frontend renders that graph and those metrics; it never invents them.

## Process boundaries

```text
┌─────────────────────────────────────────────┐
│ React application (Vite, :5173)             │
│ explorer · SQL editor · results · plan view │
└────────────────────┬────────────────────────┘
                     │ HTTP / JSON  (:8787)
┌────────────────────▼────────────────────────┐
│ API layer          internal/api             │
│ routing · CORS · errors · limits            │
├─────────────────────────────────────────────┤
│ Query service      internal/query           │
│ parse → validate → plan → execute           │
├─────────────────────────────────────────────┤
│ Operators          internal/execution       │
│ scan · filter · project · join · aggregate  │
│ sort · limit                                │
├─────────────────────────────────────────────┤
│ Catalog            internal/catalog         │
│ tables · schemas · typed rows               │
└─────────────────────────────────────────────┘
```

Data flows one way. Each layer depends on the one below it; none of them depend on HTTP request or response types.

## Backend packages

| Package | Responsibility | Phase |
| --- | --- | --- |
| `internal/config` | Environment-driven process configuration | 0 |
| `internal/api` | Routing, middleware, request/response DTOs, error mapping | 0 |
| `internal/catalog` | Tables, schemas, typed rows, registry | 1 |
| `internal/dataset` | Dataset grouping, CSV parsing, type inference, import limits | 2 |
| `internal/sqlparse` | Lexer, recursive-descent parser, QueryLens AST, syntax errors | Implemented (Phase 3) |
| `internal/validate` | Name resolution, wildcard expansion, output naming, ambiguity checks | Implemented (Phase 3) |
| `internal/planner` | Logical operator tree construction | 6 |
| `internal/execution` | Operator implementations and execution context | 5 |
| `internal/metrics` | Per-operator instrumentation and bounded samples | 7 |
| `internal/query` | Orchestrates the pipeline and assembles the response | 8 |

Only `internal/api` and `internal/query` know about HTTP. Execution operators know nothing about the browser, the graph DTOs, or JSON.

## Frontend boundaries

```text
frontend/src/
├── app/          shell, providers, routing
├── components/   ui primitives, layout, feedback
├── features/     datasets · sql-editor · results · execution-plan
│                 operator-inspector · query-history · settings
├── lib/          api client, formatting, keyboard
├── styles/       design tokens, globals
└── types/        API DTOs and view models
```

API DTOs (`types/`) are separate from view models. React Flow node objects are constructed in the execution-plan feature from backend graph data; they are not the transport format.

## Data model direction

Values are typed, not stringly. `NULL` is a distinct value, never an empty string or zero. Rows carry schema metadata so a column stays identifiable after a join introduces duplicate names.

The initial execution model is pull-based iterators, because operator composition and instrumentation are simpler to reason about than a push model. Cancellation is passed through the execution context and checked inside long-running operators.

## Current implementation status

Phases 0–3 are implemented. The API exposes health, dataset import (CSV), dataset and table metadata, and paginated row previews. Data lives in memory and is lost on restart. The frontend is a dataset explorer with schema inspection and a bounded row grid; there is no SQL editor, planner, or executor yet.

CSV import is deliberately conservative: it infers only `integer`, `decimal`, `boolean`, and `text`, treats empty fields and configured tokens as NULL, keeps identifier-like numerics (leading zeros) and int64 overflows as text, and reports adjusted headers before commit. The full contract is in [api.md](api.md) and [ADR 0003](decisions/0003-csv-import.md).

SQL text is now parsed and validated. `internal/sqlparse` is a hand-written lexer and recursive-descent parser for the subset in [sql-support.md](sql-support.md); `internal/validate` binds tables and columns, expands wildcards, and detects unknown or ambiguous names. Nothing executes a query yet, and no query endpoint exists. See [ADR 0004](decisions/0004-sql-parser.md).

## Assumptions

1. Single user, local process. No authentication, no multi-tenancy, no network database connectors.
2. In-memory datasets. Data does not survive a restart; that is acceptable until the core workflow is correct.
3. Read-only SQL. No DDL or DML is ever executed.
4. Bounded everything: SQL length, result rows, intermediate samples, imported file size. Limits are configuration, not constants scattered through handlers.
5. The browser is untrusted input. Every request is validated server-side regardless of what the UI would allow.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| SQL parser choice locks in a dialect or has a thin AST | Supported-subset work becomes adapter work | Keep parser types behind `internal/sqlparse`; the rest of the codebase sees QueryLens types only |
| Scope creep toward "real database" | Core workflow never finishes | Non-goals are explicit; unsupported syntax returns a clear error |
| Instrumentation distorts results or bloats memory | Wrong answers, OOM on wide plans | Sample limits are enforced by the recorder; instrumentation is tested to be result-neutral |
| Three-panel layout becomes cramped on laptops | Product feels unusable below 1280px | Panels collapse; tablet and mobile fall back to drawers and a list view |
| Windows-first development environment | Shell assumptions leak into scripts | npm scripts are the primary entry point; the Makefile is a mirror, not a requirement |
