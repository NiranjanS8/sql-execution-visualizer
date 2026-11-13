# API

Versioned JSON over HTTP. Base path: `/api/v1`.

The server binds to `127.0.0.1` by default and is intended for local use.

## Conventions

- Requests and responses are `application/json`.
- Every error uses one envelope shape (below). Stack traces and filesystem paths are never returned.
- Unknown fields in requests are ignored; missing required fields are rejected with `invalid_request`.
- Row limits and duration limits are enforced server-side and reported in responses when they truncate or stop work.

### Error envelope

```json
{
  "error": {
    "code": "unknown_table",
    "message": "Table \"employeez\" does not exist in dataset \"employees\".",
    "location": { "line": 1, "column": 15 }
  }
}
```

| Field | Type | Notes |
| --- | --- | --- |
| `code` | string | Stable machine-readable identifier. Never changes meaning. |
| `message` | string | Human-readable, safe to display. |
| `location` | object? | Present for parse and validation errors where a source position is known. |

Stable codes introduced so far: `invalid_request`, `not_found`, `method_not_allowed`, `internal_error`. Codes for query failures (`parse_error`, `unsupported_syntax`, `unknown_table`, `unknown_column`, `ambiguous_column`, `type_error`, `execution_error`, `timeout`, `result_limit_exceeded`) are reserved and land with Phases 3–8.

## Endpoints

### `GET /api/v1/health`

Liveness and build identity. Always cheap; never touches datasets.

```json
{
  "status": "ok",
  "service": "querylens",
  "version": "0.1.0-dev",
  "time": "2026-11-13T10:12:31Z",
  "uptimeMs": 4821
}
```

`status` is `ok` when the process can serve requests. A future revision may add a `checks` array once there is state worth checking; the field is additive, not a contract change.

### Planned

| Method | Path | Phase |
| --- | --- | --- |
| `GET` | `/api/v1/datasets` | 2 |
| `POST` | `/api/v1/datasets` | 2 |
| `GET` | `/api/v1/datasets/{datasetId}` | 2 |
| `DELETE` | `/api/v1/datasets/{datasetId}` | 2 |
| `GET` | `/api/v1/datasets/{datasetId}/tables` | 2 |
| `GET` | `/api/v1/datasets/{datasetId}/tables/{tableId}` | 2 |
| `GET` | `/api/v1/datasets/{datasetId}/tables/{tableId}/rows` | 2 |
| `POST` | `/api/v1/queries` | 8 |
| `GET` | `/api/v1/queries/{queryRunId}` | 8 |

Query responses are designed so asynchronous execution can be added later without breaking the contract: the response always carries a `queryRunId` and a `status`.

## Limits

Limits exist from the first endpoint that can consume resources. Values will be settled in the phases that need them; the categories are fixed now.

| Limit | Applies to |
| --- | --- |
| SQL length | Query requests |
| Execution duration | Query requests |
| Result rows | Query responses |
| Intermediate samples per operator | Execution graph |
| Import file size, rows, columns | Dataset import |
| Request body size | All mutating endpoints |

## CORS

Allowed origins come from `QUERYLENS_ALLOWED_ORIGINS` (comma-separated, default `http://localhost:5173`). Preflight requests are answered for `GET`, `POST`, `DELETE`, and `OPTIONS`, with `Content-Type` and `Accept` allowed.
