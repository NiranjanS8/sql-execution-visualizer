# QueryLens

A local-first, visual SQL execution engine for learning, debugging, and understanding how SQL queries transform data.

Load a dataset, write a query, run it, and see two things at once: the rows that came back, and a graph of the operators that produced them — each with its real row counts, duration, and configuration.

> **Status: Phase 6 — logical planner.** The backend imports CSV files into in-memory datasets, parses and type-checks the supported SQL subset, plans single-table queries into an inspectable operator tree, and runs them through pull-based scan, filter, sort, project, and limit operators. Unsupported syntax and invalid operand types fail loudly before execution. The frontend is a dataset explorer with schema inspection and a bounded row preview. The query API and execution graph are not implemented yet. See [docs/roadmap.md](docs/roadmap.md) and [docs/sql-support.md](docs/sql-support.md).

## Why

Most SQL tools show you an answer. QueryLens shows you the process: which operators ran, in what order, how many rows entered and left each one, and where the time went. The execution graph is the product; everything else stays quiet so the graph and the data stay legible.

## Prerequisites

| Tool | Version | Notes |
| --- | --- | --- |
| Go | 1.27+ | Backend (`backend/go.mod`) |
| Node.js | 20+ | Frontend tooling |
| npm | 10+ | Ships with Node |

`make` is optional. Every task is available as an npm script; the `Makefile` mirrors them for convenience.

## Quick start

```bash
npm run install:all   # install root + frontend dependencies
npm run dev           # start API on :8787 and web on :5173
```

Open http://localhost:5173. The status strip reports whether the API is reachable. The `samples` dataset is available immediately; use **Import CSV** in the explorer to load `samples/employees.csv` or your own file.

To run the two processes separately:

```bash
npm run dev:api       # http://127.0.0.1:8787
npm run dev:web       # http://localhost:5173
```

## Commands

| Command | What it does |
| --- | --- |
| `npm run dev` | Run API and web dev servers together |
| `npm run dev:api` | Run only the Go API |
| `npm run dev:web` | Run only the Vite dev server |
| `npm run build` | Build the API binary and the frontend bundle |
| `npm test` | Run backend and frontend tests |
| `npm run lint` | `go vet` plus frontend ESLint |
| `npm run fmt` | `gofmt` plus Prettier |
| `npm run tidy` | `go mod tidy` |
| `make <target>` | Same tasks via the Makefile (`make help` to list) |

## Configuration

The API reads environment variables at startup.

| Variable | Default | Purpose |
| --- | --- | --- |
| `QUERYLENS_ADDR` | `127.0.0.1:8787` | Listen address. Bound to loopback by default. |
| `QUERYLENS_ALLOWED_ORIGINS` | `http://localhost:5173` | Comma-separated CORS origins. |

The frontend reads build-time Vite variables.

| Variable | Default | Purpose |
| --- | --- | --- |
| `VITE_API_BASE_URL` | `http://127.0.0.1:8787` | Base URL for API requests. |

## Repository layout

```text
.
├── backend/            Go API server
│   ├── cmd/querylens/  process entry point
│   └── internal/       api, config, catalog, dataset, sqlparse, validate, eval, planner, execution
├── frontend/           React + TypeScript + Vite application
│   └── src/
│       ├── app/        shell, providers, routes
│       ├── components/ ui, layout, feedback primitives
│       ├── features/   feature-oriented modules
│       ├── lib/        api client, formatting, keyboard
│       └── styles/     design tokens and globals
├── docs/               architecture, API, SQL support, design, decisions
└── samples/            sample CSV datasets
```

## Documentation

- [Architecture](docs/architecture.md) — boundaries, data flow, and open risks
- [API](docs/api.md) — endpoints, error envelope, limits
- [SQL support](docs/sql-support.md) — what is supported, what is refused
- [Design system](docs/design/design-system.md) — tokens, typography, component inventory
- [Design references](docs/design/references.md) — the patterns this UI borrows from
- [Decisions](docs/decisions/) — architecture decision records
- [Roadmap](docs/roadmap.md) — phase plan and current position

## Principles

1. Explain execution, not just syntax.
2. Correctness before breadth.
3. Deterministic results.
4. Observable operators.
5. Local-first.
6. Incremental architecture.
7. No pretend database — unsupported SQL fails loudly instead of being approximated.

## License

Not yet decided.
