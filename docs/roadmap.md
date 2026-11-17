# Roadmap

Two plans run in parallel. The engineering plan builds the engine; the design plan builds the interface. A phase is complete only when its acceptance criteria pass and the documentation is updated.

## Current position

**Phase 0 — complete. UI Phases A, C, D, E, F — complete. Phases 1–9 — complete.** Next: Phase 10 (joins and multi-table queries).

## Engineering phases

| Phase | Scope | Status |
| --- | --- | --- |
| 0 | Repository foundation, health endpoint, frontend shell | ✅ Complete |
| 1 | Typed values, rows, schemas, in-memory catalog, sample datasets | ✅ Complete |
| 2 | CSV import, dataset browser API and UI | ✅ Complete |
| 3 | SQL parser adapter and supported-syntax validation | ✅ Complete |
| 4 | Expression evaluation with three-valued NULL logic | ✅ Complete |
| 5 | Execution operators: scan, filter, project, limit, sort | ✅ Complete |
| 6 | Logical planner and end-to-end single-table queries | ✅ Complete |
| 7 | Execution recorder and plan metadata | ✅ Complete |
| 8 | Query API and result presentation | ✅ Complete |
| 9 | Interactive execution-plan visualization | ✅ Complete |
| 10 | Joins and multi-table queries | Next |
| 11 | Aggregation and grouping | — |
| 12 | SQL editor and product polish | — |
| 13 | Reliability, security, performance, release | — |

## UI phases

| Phase | Scope | Status |
| --- | --- | --- |
| A | Design discovery: direction, tokens, wireframes, component inventory | ✅ Complete |
| B | Application shell and design system | Partial — shell and tokens exist, panels do not |
| C | Dataset explorer | ✅ Complete |
| D | SQL editor | ✅ Complete |
| E | Results grid | ✅ Complete |
| F | Execution graph and operator inspector | ✅ Complete |
| G | History, settings, polish | Next |
| H | Visual QA and release readiness | — |

## Sequencing notes

- UI Phase A must precede component work, which is why it landed with Phase 0.
- UI Phase B's resizable panels are deferred until there are at least two panels worth resizing (UI Phase C/D). Building a three-pane splitter with placeholder content would produce dead controls.
- Phase 3 (parser) and UI Phase D (editor) met at the query API in Phase 8, where the editor became the first real client of `POST /queries`.
