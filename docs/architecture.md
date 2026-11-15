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
│ parse → validate → plan → execute → trace   │
├─────────────────────────────────────────────┤
│ Recorder           internal/metrics         │
│ IDs · durations · row counts · samples      │
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
| `internal/validate` | Name resolution, wildcard expansion, output naming/types, type checks | Implemented (Phases 3–4) |
| `internal/eval` | Expression type inference, evaluation, three-valued NULL logic | Implemented (Phase 4) |
| `internal/planner` | Logical plan tree, operator ordering, explain output, operator mapping | Implemented (Phase 6) |
| `internal/execution` | Pull-based operators (scan, filter, sort, project, limit) and cancellation | Implemented (Phase 5) |
| `internal/metrics` | Per-run operator IDs, durations, row counts, bounded samples, statuses | Implemented (Phase 7) |
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

Expressions are type-checked before they run, and `NULL` follows SQL three-valued logic rather than ordinary boolean behaviour. The rules are in [ADR 0005](decisions/0005-expression-semantics.md).

The execution model is pull-based iterators, because operator composition and instrumentation are simpler to reason about than a push model. Cancellation is passed through the execution context and checked inside long-running operators. Scan, filter, project, and limit stream; sort materializes its input so it can order it stably. See [ADR 0006](decisions/0006-execution-operators.md).

## Current implementation status

Phases 0–7 are implemented. The API exposes health, dataset import (CSV), dataset and table metadata, and paginated row previews. Data lives in memory and is lost on restart. The frontend is a dataset explorer with schema inspection and a bounded row grid; there is no SQL editor, query endpoint, or execution graph yet.

CSV import is deliberately conservative: it infers only `integer`, `decimal`, `boolean`, and `text`, treats empty fields and configured tokens as NULL, keeps identifier-like numerics (leading zeros) and int64 overflows as text, and reports adjusted headers before commit. The full contract is in [api.md](api.md) and [ADR 0003](decisions/0003-csv-import.md).

SQL text is parsed, validated, type-checked, planned, and executable. `internal/sqlparse` is a hand-written lexer and recursive-descent parser for the subset in [sql-support.md](sql-support.md); `internal/validate` binds tables and columns, expands wildcards, detects unknown or ambiguous names, and rejects incompatible operand types; `internal/eval` infers output types and evaluates expressions with SQL three-valued NULL logic; `internal/planner` turns a validated query into an inspectable logical plan (`Scan → Filter → Sort → Project → Limit`) and maps it to operators; `internal/execution` runs that operator chain; `internal/metrics` wraps the chain per run and records stable IDs, exclusive durations, input/output row counts, bounded samples, and per-operator statuses. There is no query endpoint yet, so execution and its trace are only reachable from tests. See [ADR 0004](decisions/0004-sql-parser.md), [ADR 0005](decisions/0005-expression-semantics.md), [ADR 0006](decisions/0006-execution-operators.md), [ADR 0007](decisions/0007-logical-planner.md), and [ADR 0008](decisions/0008-execution-recorder.md).

## Assumptions

1. Single user, local process. No authentication, no multi-tenancy, no network database connectors.
2. In-memory datasets. Data does not survive a restart; that is acceptable until the core workflow is correct.
3. Read-only SQL. No DDL or DML is ever executed.
4. Bounded everything: SQL length, result rows, intermediate samples, imported file size. Limits are configuration, not constants scattered through handlers.
5. The browser is untrusted input. Every request is validated server-side regardless of what the UI would allow.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| SQL parser choice locks in a dialect or has a thin AST | Supported-subset work becomes adapter work | The grammar and AST stay in `internal/sqlparse`; the rest of the codebase sees QueryLens types only |
| Scope creep toward "real database" | Core workflow never finishes | Non-goals are explicit; unsupported syntax returns a clear error |
| Instrumentation distorts results or bloats memory | Wrong answers, OOM on wide plans | Sample limits are enforced by the recorder; instrumentation is tested to be result-neutral |
| Three-panel layout becomes cramped on laptops | Product feels unusable below 1280px | Panels collapse; tablet and mobile fall back to drawers and a list view |
| Windows-first development environment | Shell assumptions leak into scripts | npm scripts are the primary entry point; the Makefile is a mirror, not a requirement |
