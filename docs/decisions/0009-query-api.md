# 0009 — Query API: synchronous execution, limits, and the trace contract

**Status:** Accepted
**Date:** 2026-11-15
**Phase:** 8

## Context

Phases 3–7 build a complete pipeline — parse, resolve, plan, execute, instrument — but nothing outside the Go tests can reach it. Phase 8 must expose one request that runs a supported SELECT and returns both the rows and the execution trace, define the wire contract for that trace, enforce budgets on SQL length, execution time, and result size, classify every failure into a stable error code, and let the browser cancel a run it no longer wants.

The UI must never invent execution data. Whatever the endpoint returns is the whole truth the frontend may render, so the contract has to carry the trace and the result together, and every limit has to fail loudly rather than quietly trim.

## Decision

### One synchronous endpoint

`POST /api/v1/queries` accepts `{ "datasetId": "...", "sql": "..." }` and returns the completed run in one response. The dataset scopes the catalog: the SQL resolves only against that dataset's tables, so two datasets that both contain `employees` never shadow each other.

Execution is synchronous because the current engine is fast on in-memory data and the frontend only needs one answer per submission. `internal/query.Runner.Execute` owns the pipeline order (`parse → resolve → plan → recorder → drain`); `internal/api` owns transport only.

The response always carries a `queryRunId` and a `status`, even though no route reads a run back yet. That keeps the shape ready for asynchronous execution and `GET /api/v1/queries/{queryRunId}` without a contract change.

### Response shape

- `queryRunId` — a per-process run identifier; reserved for future retrieval.
- `status` — the run status. A `200` response currently reports `completed`; failures use the error envelope. The field is reserved for `failed` or `cancelled` once runs can be persisted or executed asynchronously.
- `datasetId` — echoes the scope the query resolved against.
- `columns` — ordered result columns (`id`, `name`, `type`). They are result columns, not table columns, so they carry no nullability or table identity.
- `rows` — arrays aligned with `columns`. Text is a string, integer and decimal are base-10 strings, boolean is a JSON bool, and SQL NULL is `null`.
- `rowCount` and `resultLimit` — rows returned and the budget that was in force.
- `durationMs` — total pipeline time, from parsing to the last row; it excludes response encoding. It is a number, not an integer, so durations are not rounded to whole milliseconds; the resolution still depends on the platform clock.
- `execution` — `rootId`, `nodes`, `edges`, `durationMs`, `status`, and optional `error`.

`execution.nodes` is the recorder's trace mapped to JSON. Each node exposes its stable ID, kind, label, parent and children, output schema, rows in/out, exclusive duration, status, optional error, and bounded input/output samples. A node's input sample uses its child's schema; a scan has no child, so its input columns are empty. `execution.rootId` names the node whose rows are the result, and `execution.edges` repeats the parent/child links as parent-to-child pairs.

The full response for a three-operator query is in [api.md](../api.md#post-apiv1queries).

The DTO is deliberately separate from `metrics.Trace`; the Go trace can change without silently changing the wire format.

### Limits

| Limit | Default | Environment variable |
| --- | --- | --- |
| SQL length | 10,000 characters | `QUERYLENS_QUERY_MAX_SQL_LENGTH` |
| Execution time | 5 s | `QUERYLENS_QUERY_TIMEOUT` |
| Result rows | 1,000 | `QUERYLENS_QUERY_MAX_ROWS` |
| Sample rows per operator | 5 | `QUERYLENS_QUERY_SAMPLE_LIMIT` |

SQL length is counted in runes, not bytes, so a multi-byte character costs one position. The timeout wraps the request context, so it composes with client cancellation instead of replacing it.

The result limit is enforced by pulling one row past the budget. Exactly at the limit succeeds; one more row fails with `result_limit_exceeded`. The extra row is discarded and never returned as a partial result — there is no silent truncation.

Samples are bounded separately from results. A query may return 1,000 rows while each operator keeps at most five for inspection, and `truncated` says whether rows flowed that were not retained.

### Error mapping

Every query failure uses the existing envelope and adds `location` when a source position is known.

| Code | Status | Raised by |
| --- | --- | --- |
| `invalid_request` | 400 | Missing `datasetId`, malformed JSON, SQL over the length limit |
| `parse_error` | 400 | `internal/sqlparse` |
| `unsupported_syntax` | 400 | `internal/sqlparse`, `internal/validate` |
| `unknown_table` | 400 | `internal/validate` |
| `unknown_column` | 400 | `internal/validate` |
| `ambiguous_column` | 400 | `internal/validate` |
| `type_error` | 400 | `internal/eval`, `internal/validate`, `internal/execution` |
| `dataset_not_found` | 404 | The request's dataset is not registered |
| `execution_error` | 422 | `internal/eval` at row time (division by zero, overflow) |
| `result_limit_exceeded` | 422 | `internal/query` |
| `timeout` | 408 | Execution exceeded the configured deadline |
| `cancelled` | 408 | The client aborted the request |
| `internal_error` | 500 | Unexpected failure; details are logged, not returned |

Parse, resolution, and type failures are distinguishable by code and status: they are the user's SQL and arrive before any operator runs. An `execution_error` is a runtime failure against actual data. A `timeout` or `cancelled` describes the run being stopped, not a defect in the query.

`internal/query.Error` carries the stable code, message, and optional line/column, and wraps an internal cause only for unexpected execution failures. The API logs that cause and returns a generic message.

### Cancellation

The browser aborts the `fetch` when the user cancels or starts another run. The request context is passed into the operator chain, which checks it before producing a row; sort also checks while it materializes input. Operators return `context.Canceled`/`context.DeadlineExceeded` unchanged, so `internal/query` can classify them.

If the client is already gone, the API suppresses the response instead of writing an error nobody will read. `cancelled` is an additive stable code for the cases where a response can still be delivered.

### Deferred retrieval route

`GET /api/v1/queries/{queryRunId}` is not implemented. Nothing persists a run today, so the route would have to either re-execute the query (different metrics) or fabricate a result (forbidden). It is documented as deferred until results are stored for asynchronous execution; the response shape already reserves `queryRunId` and `status` for that future.

## Alternatives considered

| Alternative | Why rejected |
| --- | --- |
| Separate endpoints for rows and trace | Two requests can observe different runs; the trace would not describe the returned rows |
| Return only the result rows and compute the trace in the browser | The frontend cannot know row counts, durations, or samples; it would have to invent them |
| Silently truncate results at the limit | Hides that the answer is incomplete; the product promises loud failure |
| Treat the result limit as a `LIMIT` rewrite | Changes the query's meaning and hides that the original result was larger |
| Store runs and return `202 Accepted` now | Adds persistence and polling before there is a need; synchronous execution is fast enough |
| Map `timeout` and `cancelled` to 500 | They describe a stopped run, not a server defect |
| Reuse `metrics.Trace` directly as the JSON DTO | Couples the internal trace to the wire format and leaks Go field names |

## Consequences

- `internal/query` is the only package that composes the SQL pipeline; handlers stay thin.
- `POST /api/v1/queries` is the first route that returns execution data, and the frontend's `Query` tab consumes it.
- Limits live in configuration with documented defaults; invalid environment values fall back to the defaults rather than failing startup.
- `query.Runner` is safe for concurrent use; the only shared state is an atomic run counter. Run IDs are `run-<UnixNano>-<counter>` and are unique per process, not across restarts.
- `query.DefaultLimits()` and `internal/config` intentionally duplicate the default numbers. The config values feed the runner; the runner's defaults keep the package usable in tests without config.
- No run is persisted. Refreshing the page loses results and traces, which is acceptable until query history exists.
- The response is additive-friendly: new node fields, sample metadata, or a persisted-run route can be added without breaking existing clients.

## Tests

`backend/internal/query/query_test.go` covers:

- result rows, column IDs/names/types, trace root and node/edge counts;
- bounded samples and truncation, including the scan sample;
- parse, unknown-table, unknown-column, and type errors;
- runtime execution errors with a source position;
- SQL length, result limit, exactly-at-the-limit, and `LIMIT 0`;
- timeout and cancellation classification;
- unique run IDs and default application.

`backend/internal/api/queries_test.go` covers:

- the full response shape, result cells, and trace/sample mapping;
- NULL versus empty string encoding;
- structured parse/resolution/type/execution errors and their statuses;
- SQL length, result limit, timeout, client-cancellation suppression, malformed JSON, missing and unknown dataset, oversized body;
- empty arrays for zero-row results and `405` for unsupported methods.
