# API

Versioned JSON over HTTP. Base path: `/api/v1`.

The server binds to `127.0.0.1` by default and is intended for local use.

## Conventions

- Responses are `application/json`, except `DELETE`, which returns `204` with no body.
- Imports use `multipart/form-data`; query execution sends a JSON body; every other request has no body.
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
| `parse_error` | 400 | The SQL is not valid in the supported grammar. |
| `unsupported_syntax` | 400 | Valid SQL, but outside the supported subset. |
| `unknown_table` | 400 | The `FROM` table or a qualifier does not resolve. |
| `unknown_column` | 400 | A column reference does not resolve. |
| `ambiguous_column` | 400 | A column reference matches more than one source. |
| `type_error` | 400 | An operand type is invalid for its operator, or `WHERE`/`ORDER BY` has the wrong type. |
| `execution_error` | 422 | Evaluation failed at row time: division by zero, numeric overflow, or an unbound column. |
| `result_limit_exceeded` | 422 | The query produced more rows than the result limit. |
| `timeout` | 408 | Execution exceeded the configured deadline. |
| `cancelled` | 408 | The client cancelled the request. |
| `not_found` | 404 | No route matches the path. |
| `method_not_allowed` | 405 | The route exists but not for that method. |
| `internal_error` | 500 | Unexpected server failure. Details are logged, not returned. |

Query codes carry `location` when the parser, validator, or evaluator knows a source position.

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

## Queries

### `POST /api/v1/queries`

Runs one supported `SELECT` against one dataset and returns the result rows together with the execution trace.

```json
{
  "datasetId": "samples",
  "sql": "SELECT name, salary FROM employees LIMIT 3"
}
```

`datasetId` scopes the catalog: the SQL resolves only against that dataset's tables. `sql` must be a single `SELECT` in the subset described by [sql-support.md](sql-support.md).

The example below runs against the built-in `samples` dataset, whose `employees` table has columns `id`, `name`, `department`, `salary`, `hire_date`, and `manager_id`. Durations are illustrative.

```json
{
  "queryRunId": "run-1794758112555100900-1",
  "status": "completed",
  "datasetId": "samples",
  "columns": [
    { "id": "c1", "name": "name", "type": "text" },
    { "id": "c2", "name": "salary", "type": "integer" }
  ],
  "rows": [
    ["Ada Lovelace", "145000"],
    ["Grace Hopper", "132000"],
    ["Alan Turing", "151000"]
  ],
  "rowCount": 3,
  "resultLimit": 1000,
  "durationMs": 0.61,
  "execution": {
    "rootId": "n1",
    "nodes": [
      {
        "id": "n1",
        "kind": "limit",
        "label": "3",
        "children": ["n2"],
        "columns": [
          { "id": "c1", "name": "name", "type": "text" },
          { "id": "c2", "name": "salary", "type": "integer" }
        ],
        "rowsIn": 3,
        "rowsOut": 3,
        "durationMs": 0.02,
        "status": "completed",
        "input": {
          "columns": [
            { "id": "c1", "name": "name", "type": "text" },
            { "id": "c2", "name": "salary", "type": "integer" }
          ],
          "rows": [
            ["Ada Lovelace", "145000"],
            ["Grace Hopper", "132000"],
            ["Alan Turing", "151000"]
          ],
          "truncated": false
        },
        "output": {
          "columns": [
            { "id": "c1", "name": "name", "type": "text" },
            { "id": "c2", "name": "salary", "type": "integer" }
          ],
          "rows": [
            ["Ada Lovelace", "145000"],
            ["Grace Hopper", "132000"],
            ["Alan Turing", "151000"]
          ],
          "truncated": false
        }
      },
      {
        "id": "n2",
        "kind": "project",
        "label": "name, salary",
        "parentId": "n1",
        "children": ["n3"],
        "columns": [
          { "id": "c1", "name": "name", "type": "text" },
          { "id": "c2", "name": "salary", "type": "integer" }
        ],
        "rowsIn": 3,
        "rowsOut": 3,
        "durationMs": 0.02,
        "status": "completed",
        "input": {
          "columns": [
            { "id": "c1", "name": "id", "type": "integer" },
            { "id": "c2", "name": "name", "type": "text" },
            { "id": "c3", "name": "department", "type": "text" },
            { "id": "c4", "name": "salary", "type": "integer" },
            { "id": "c5", "name": "hire_date", "type": "text" },
            { "id": "c6", "name": "manager_id", "type": "integer" }
          ],
          "rows": [
            ["1", "Ada Lovelace", "Engineering", "145000", "2019-03-04", null],
            ["2", "Grace Hopper", "Engineering", "132000", "2018-07-16", "1"],
            ["3", "Alan Turing", "Research", "151000", "2017-01-09", "1"]
          ],
          "truncated": false
        },
        "output": {
          "columns": [
            { "id": "c1", "name": "name", "type": "text" },
            { "id": "c2", "name": "salary", "type": "integer" }
          ],
          "rows": [
            ["Ada Lovelace", "145000"],
            ["Grace Hopper", "132000"],
            ["Alan Turing", "151000"]
          ],
          "truncated": false
        }
      },
      {
        "id": "n3",
        "kind": "scan",
        "label": "employees",
        "parentId": "n2",
        "children": [],
        "columns": [
          { "id": "c1", "name": "id", "type": "integer" },
          { "id": "c2", "name": "name", "type": "text" },
          { "id": "c3", "name": "department", "type": "text" },
          { "id": "c4", "name": "salary", "type": "integer" },
          { "id": "c5", "name": "hire_date", "type": "text" },
          { "id": "c6", "name": "manager_id", "type": "integer" }
        ],
        "rowsIn": 0,
        "rowsOut": 3,
        "durationMs": 0.05,
        "status": "completed",
        "input": {
          "columns": [],
          "rows": [],
          "truncated": false
        },
        "output": {
          "columns": [
            { "id": "c1", "name": "id", "type": "integer" },
            { "id": "c2", "name": "name", "type": "text" },
            { "id": "c3", "name": "department", "type": "text" },
            { "id": "c4", "name": "salary", "type": "integer" },
            { "id": "c5", "name": "hire_date", "type": "text" },
            { "id": "c6", "name": "manager_id", "type": "integer" }
          ],
          "rows": [
            ["1", "Ada Lovelace", "Engineering", "145000", "2019-03-04", null],
            ["2", "Grace Hopper", "Engineering", "132000", "2018-07-16", "1"],
            ["3", "Alan Turing", "Research", "151000", "2017-01-09", "1"]
          ],
          "truncated": false
        }
      }
    ],
    "edges": [
      { "from": "n1", "to": "n2" },
      { "from": "n2", "to": "n3" }
    ],
    "durationMs": 0.34,
    "status": "completed"
  }
}
```

| Field | Meaning |
| --- | --- |
| `queryRunId` | Identifier for this run. Unique per process; reserved for future retrieval. |
| `status` | Run status. A `200` response currently reports `completed`; failures use the error envelope instead. The field is reserved so persisted or asynchronous runs can report `failed` or `cancelled` later. |
| `columns` | Result columns in order. Result columns have no nullability or table identity. |
| `rows` | Result rows, encoded as described in [Cell encoding](#cell-encoding). |
| `rowCount` | Number of rows returned. |
| `resultLimit` | The result row budget that was in force. |
| `durationMs` | Total pipeline time for the run, from parsing to the last row; excludes response encoding. |
| `execution` | The execution trace: root ID, nodes, edges, duration, status, optional error. |

Each execution node carries:

| Field | Meaning |
| --- | --- |
| `id` | Stable per-run ID, `n1`…`nN` in pre-order from the root. |
| `kind` | `scan`, `filter`, `sort`, `project`, or `limit`. |
| `label` | Human-readable configuration, such as the projected columns or limit value. |
| `parentId` | Present for every non-root node. |
| `children` | Child node IDs in data-flow order. |
| `columns` | The schema of the rows the node emits. |
| `rowsIn` / `rowsOut` | Rows successfully pulled from the child and returned to the parent. |
| `durationMs` | Exclusive wall-clock time inside the node, excluding time waiting on its child. |
| `status` | `not_started`, `running`, `completed`, `stopped`, `failed`, or `cancelled`. |
| `error` | Present when the node observed a failure. |
| `input` / `output` | Bounded row samples, each with `columns`, `rows`, and `truncated`. |

A node's input sample uses its child's schema; a scan has no child, so its input columns are empty. `truncated` is true when rows flowed but were not retained.

Failures use the standard error envelope. Parse, resolution, and type errors are `400`; runtime evaluation and result-limit failures are `422`; a timeout or cancellation is `408`. The full mapping is in the error table above and in [ADR 0009](decisions/0009-query-api.md).

### Retrieval — planned, not implemented

`GET /api/v1/queries/{queryRunId}` is deferred until runs are persisted. Nothing stores a run today, and the server will not re-execute a query or fabricate a trace to satisfy a lookup. The response shape already carries `queryRunId` and `status` so retrieval can be added without a contract break.

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
| `GET` | `/api/v1/queries/{queryRunId}` | Deferred until runs are persisted |

## Limits

| Limit | Value | Applies to |
| --- | --- | --- |
| Import file size | 5 MiB | `POST /datasets`, `POST /datasets/preview` |
| Import rows | 100,000 | same |
| Import columns | 100 | same |
| Row page size | 50 default, 200 max | `GET …/rows` |
| Preview rows | 50 | `POST /datasets/preview` |
| Query request body | 64 KiB | `POST /queries` |
| SQL length | 10,000 characters | `POST /queries` |
| Execution duration | 5 s | `POST /queries` |
| Result rows | 1,000 | `POST /queries` |
| Intermediate samples per operator | 5 rows | `POST /queries` execution trace |

The import budget is defined in `backend/internal/dataset/csv.go`; the query budgets are in `backend/internal/config/config.go` and can be overridden with `QUERYLENS_QUERY_MAX_SQL_LENGTH`, `QUERYLENS_QUERY_TIMEOUT`, `QUERYLENS_QUERY_MAX_ROWS`, and `QUERYLENS_QUERY_SAMPLE_LIMIT`. Exceeding a limit is an error, never a silent truncation: exactly 1,000 result rows succeeds, and 1,001 fails with `result_limit_exceeded`.

## CORS

Allowed origins come from `QUERYLENS_ALLOWED_ORIGINS` (comma-separated, default `http://localhost:5173`). Preflight requests are answered for `GET`, `POST`, `DELETE`, and `OPTIONS`, with `Content-Type` and `Accept` allowed.
