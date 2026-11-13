# 0003 — CSV import and dataset model

**Status:** Accepted
**Date:** 2026-11-13
**Phase:** 2

## Context

Phase 2 turns an uploaded file into something the query engine can scan. The engineering plan requires header handling, conservative and deterministic type inference, configurable NULL conventions, malformed-record and row-width validation, and hard file/row/column limits. The design plan adds a rule: inference is shown before commit and never silently applied.

The domain model from Phase 1 already has typed values, schemas, stable column IDs, and a table registry. Import must produce a valid `catalog.Table` or fail with an error the user can act on.

## Decision

### One import creates one dataset with one table

The store groups tables under a dataset. A CSV upload creates a new dataset containing exactly one table. Dataset and table names both default to the sanitized filename stem; both can be overridden in the import form.

Adding a table to an existing dataset is supported by the store (`Create` is variadic) but not exposed over HTTP yet. Doing so would require deciding how duplicate table names within a dataset are handled, which is not needed to satisfy the phase.

Dataset IDs are slugs of the display name, made unique with `-2`, `-3`, …. `preview` is a reserved route literal and is never generated as an ID, so `POST /datasets/preview` cannot shadow a real dataset.

### Conservative inference, widest type wins

Each column is scanned in full. The inferred type is the first of:

1. `integer` — every non-NULL value is a base-10 int64 literal.
2. `decimal` — every non-NULL value is a finite numeric literal.
3. `boolean` — every non-NULL value is `true` or `false`, case-insensitive.
4. `text` — anything else, including a column with no non-NULL values.

Two deliberate exclusions keep inference from fabricating precision:

- **Leading zeros stay text.** `007` is an identifier, not the number seven.
- **Integers that overflow int64 stay text.** They must not fall through to `decimal`, where binary64 would silently round them.

A column that mixes formats becomes `text` and produces a warning naming the column, so the choice is visible in the preview rather than surprising after import.

Dates and timestamps remain `text`. There is no date type in the value model, and guessing one from a string would be a new feature with its own semantics (time zones, formats, comparison), not an inference detail.

### NULL conventions

An empty field is always NULL. By default `NULL` and `\N` are NULL too. Additional tokens can be supplied per import; they are merged with the defaults, matched case-insensitively. Fields are trimmed of surrounding whitespace before matching and conversion.

Trimming is a real trade-off: a quoted field containing significant leading spaces loses them. It is accepted because hand-written CSVs and spreadsheet exports are the primary input, and the preview shows the trimmed values before commit.

### Headers are sanitized, deduplicated, and reported

Headers become identifiers matching `^[A-Za-z_][A-Za-z0-9_]*$`. Invalid characters become underscores, leading digits gain an underscore, empty headers become `column_N`, and duplicate names get `_2`, `_3`, … case-insensitively.

None of this happens silently: every adjustment appears in the preview's `warnings` array. With `header=false`, columns are named `column_1`…`column_N`.

### Limits fail loudly

| Limit | Value |
| --- | --- |
| File size | 5 MiB |
| Data rows | 100,000 |
| Columns | 100 |

Exceeding any limit returns `invalid_csv` (or `file_too_large` for the request body). Nothing is truncated and imported as if it were complete.

### Errors carry a source position

`dataset.ImportError` carries a 1-based line and column. The API maps it to `invalid_csv` with an `error.location` object, so the UI can say `Row 47: expected 4 fields, found 5.` Parse errors from `encoding/csv` keep their position.

### Numeric cells are JSON strings

`integer` and `decimal` cells are serialized as strings, not JSON numbers. JavaScript numbers cannot represent every int64 exactly, and the project forbids fabricated data. The column type tells the client how to interpret the string. NULL is JSON `null`; `""` is a distinct text value.

### Preview parses without storing

`POST /api/v1/datasets/preview` runs the same parser and limits as create, then returns inferred types, warnings, and at most 50 rows. The frontend uploads the file again on commit. This is two uploads of a bounded 5 MiB file on a loopback connection, which is simpler than holding uncommitted server-side import sessions and their expiry.

## Consequences

- Import memory is bounded by the file limit, not by a row count alone: the parser reads the whole file into memory. This is acceptable at 5 MiB and is the reason the limit exists.
- Inference scans the full file, so a preview costs the same parse as an import.
- `decimal` is still binary64, so decimal values may round. That debt is unchanged and still due in Phase 4.
- Dates stay text until a date type is designed.
- Duplicate headers are renamed rather than rejected. The preview makes this visible; rejecting would be defensible too, but renaming matches spreadsheet expectations.
- The dataset list endpoint returns tables inline. With one table per import and a handful of datasets this is one request instead of N+1; it should be revisited if datasets grow large.
- Adding a table to an existing dataset remains unexposed.

## Tests

`backend/internal/dataset/csv_test.go` covers inference for every type, leading zeros, int64 overflow, NULL tokens and overrides, header sanitization and deduplication, missing headers, row-width errors with line numbers, empty/invalid-UTF-8/NUL files, unterminated quotes, every limit, BOM stripping, quoted commas, whitespace trimming, mixed-format warnings, header-only files, and table naming.

`backend/internal/dataset/store_test.go` covers creation, ordering, unique IDs, the reserved `preview` ID, rejection of invalid tables without partial registration, deletion, and the seeded sample dataset.

`backend/internal/api/datasets_test.go` covers the list/create/get/delete routes, preview without commit, type inference over HTTP, NULL and numeric cell encoding, pagination and its bounds, invalid CSV with a location, oversized uploads, missing file parts, unknown dataset/table, 405 responses, and literal-route precedence over `{datasetID}`.
