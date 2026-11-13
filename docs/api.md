# API

Versioned JSON over HTTP. Base path: `/api/v1`.

The server binds to `127.0.0.1` by default and is intended for local use.

## Conventions

- Responses are `application/json`, except `DELETE`, which returns `204` with no body.
- Imports use `multipart/form-data`; every other request has no body.
- Every error uses one envelope shape (below). Stack traces and filesystem paths are never returned.
- Unknown form fields are ignored; missing required parts are rejected with `invalid_request`.
- Limits are enforced server-side and reported in responses when they bound a result.

### Error envelope

```json
{
  "error": {
    "code": "invalid_csv",
    "message": "expected 4 fields, found 5",
    "location": { "line": 47, "column": 5 }
  }
}
```

| Field | Type | Notes |
| --- | --- | --- |
| `code` | string | Stable machine-readable identifier. Never changes meaning. |
| `message` | string | Human-readable, safe to display. |
| `location` | object? | Present when a source position is known. `line` and `column` are 1-based and individually omitted when zero. |

Stable codes:

| Code | Status | Meaning |
| --- | --- | --- |
| `invalid_request` | 400 | The request is malformed or a parameter is out of range. |
| `invalid_csv` | 400 | The uploaded file could not be parsed or inferred. |
| `file_too_large` | 413 | The upload exceeds the import byte limit. |
| `dataset_not_found` | 404 | No dataset has the given ID. |
| `table_not_found` | 404 | The dataset exists but has no such table. |
| `not_found` | 404 | No route matches the path. |
| `method_not_allowed` | 405 | The route exists but not for that method. |
| `internal_error` | 500 | Unexpected server failure. Details are logged, not returned. |

Reserved for query work (Phases 3–8): `parse_error`, `unsupported_syntax`, `unknown_table`, `unknown_column`, `ambiguous_column`, `type_error`, `execution_error`, `timeout`, `result_limit_exceeded`.

### Cell encoding

Rows are arrays of cells aligned with the response's `columns`. A cell is one of:

| Column type | JSON cell |
| --- | --- |
| `text` | string |
| `integer` | string, base-10 (for example `"145000"`) |
| `decimal` | string, decimal notation (for example `"430.5"`) |
| `boolean` | `true` or `false` |
| any, SQL NULL | `null` |

Numeric values are strings on purpose: a 64-bit integer or a high-precision decimal would lose digits if it travelled as a JavaScript number. The column type says how to interpret the string. `null` and `""` are always distinct.

## Datasets

### `GET /api/v1/datasets`

All datasets in creation order, with their tables.

```json
{
  "datasets": [
    {
      "id": "samples",
      "name": "samples",
      "createdAt": "2026-11-13T10:12:31Z",
      "source": { "filename": "built-in", "bytes": 0 },
      "tableCount": 2,
      "rowCount": 12,
      "tables": [
        { "id": "departments", "name": "departments", "rowCount": 4, "columnCount": 4 },
        { "id": "employees", "name": "employees", "rowCount": 8, "columnCount": 5 }
      ]
    }
  ]
}
```

### `GET /api/v1/datasets/{datasetId}`

One dataset, in the same shape as a list entry. `404 dataset_not_found` when absent.

### `DELETE /api/v1/datasets/{datasetId}`

Removes the dataset and its tables from the session. Returns `204`. `404 dataset_not_found` when absent.

### `POST /api/v1/datasets`

Imports one CSV file as a new dataset containing one table. Returns `201` with the created dataset.

Multipart parts:

| Part | Required | Meaning |
| --- | --- | --- |
| `file` | yes | The CSV file. |
| `header` | no | `true` (default) treats the first record as column names; `false` generates `column_1`… |
| `tableName` | no | Overrides the name derived from the filename. Sanitized to a valid identifier. |
| `datasetName` | no | Display name. Defaults to the table name. The ID is derived from it and made unique. |
| `nullValues` | no | Comma-separated extra tokens treated as NULL, in addition to empty fields, `NULL`, and `\N`. |

### `POST /api/v1/datasets/preview`

Same multipart input as the create endpoint, but parses without storing anything. The frontend uses it to show inferred types before commit.

```json
{
  "tableName": "employees",
  "columns": [
    { "name": "id", "type": "integer", "nullability": "not_null", "nullCount": 0 },
    { "name": "manager_id", "type": "integer", "nullability": "nullable", "nullCount": 1 }
  ],
  "rows": [["1", "Ada Lovelace"]],
  "rowCount": 8,
  "byteCount": 427,
  "rowLimit": 50,
  "warnings": [],
  "nullValues": ["", "NULL", "\\N"]
}
```

`rows` is bounded by `rowLimit`. `warnings` explains adjusted headers and mixed-format columns; it is always an array, never `null`.

## Tables

### `GET /api/v1/datasets/{datasetId}/tables`

```json
{
  "tables": [{ "id": "employees", "name": "employees", "rowCount": 8, "columnCount": 5 }]
}
```

### `GET /api/v1/datasets/{datasetId}/tables/{tableId}`

Adds the ordered schema:

```json
{
  "id": "employees",
  "name": "employees",
  "rowCount": 8,
  "columnCount": 5,
  "columns": [
    { "id": "c1", "name": "id", "type": "integer", "nullability": "not_null" },
    { "id": "c2", "name": "name", "type": "text", "nullability": "not_null" },
    { "id": "c3", "name": "salary", "type": "integer", "nullability": "nullable" }
  ]
}
```

### `GET /api/v1/datasets/{datasetId}/tables/{tableId}/rows`

Paginated rows.

| Query | Default | Bounds |
| --- | --- | --- |
| `offset` | `0` | non-negative integer |
| `limit` | `50` | 1–200; larger values are clamped |

```json
{
  "tableId": "employees",
  "columns": [{ "id": "c1", "name": "id", "type": "integer", "nullability": "not_null" }],
  "rows": [["1"]],
  "offset": 0,
  "limit": 50,
  "totalRows": 8,
  "hasMore": true
}
```

`hasMore` is computed from the page, not from `offset + limit`. An offset past the end returns an empty page with `offset` clamped to `totalRows`.

## Health

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

## Planned

| Method | Path | Phase |
| --- | --- | --- |
| `POST` | `/api/v1/queries` | 8 |
| `GET` | `/api/v1/queries/{queryRunId}` | 8 |

Query responses are designed so asynchronous execution can be added later without breaking the contract: the response always carries a `queryRunId` and a `status`.

## Limits

| Limit | Value | Applies to |
| --- | --- | --- |
| Import file size | 5 MiB | `POST /datasets`, `POST /datasets/preview` |
| Import rows | 100,000 | same |
| Import columns | 100 | same |
| Row page size | 50 default, 200 max | `GET …/rows` |
| Preview rows | 50 | `POST /datasets/preview` |
| SQL length | not yet set | Query requests |
| Execution duration | not yet set | Query requests |
| Result rows | not yet set | Query responses |
| Intermediate samples per operator | not yet set | Execution graph |

The import budget is defined once in `backend/internal/dataset/csv.go` (`DefaultLimits`). Exceeding a limit is an error, never a silent truncation.

## CORS

Allowed origins come from `QUERYLENS_ALLOWED_ORIGINS` (comma-separated, default `http://localhost:5173`). Preflight requests are answered for `GET`, `POST`, `DELETE`, and `OPTIONS`, with `Content-Type` and `Accept` allowed.
