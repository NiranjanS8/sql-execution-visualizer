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
| `internal/catalog` | Datasets, tables, schemas, typed rows | 1 |
| `internal/dataset` | CSV parsing and type inference | 2 |
| `internal/sqlparse` | Parser-library adapter; converts to QueryLens AST | 3 |
| `internal/validate` | Name resolution, type checking, supported-syntax checks | 3–4 |
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

Phase 0 only. The API exposes `GET /api/v1/health`. There is no catalog, parser, planner, or executor. The frontend is a shell that reports API reachability.

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
