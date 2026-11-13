package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func doMultipart(
	t *testing.T,
	handler http.Handler,
	path string,
	fields map[string]string,
	filename string,
	content []byte,
) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if filename != "" {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create file part: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("write file part: %v", err)
		}
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field %q: %v", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func decodeBody[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var body T
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return body
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) errorEnvelope {
	t.Helper()
	return decodeBody[errorEnvelope](t, recorder)
}

func TestListDatasetsIncludesSamples(t *testing.T) {
	recorder := doRequest(t, newTestHandler(), http.MethodGet, "/api/v1/datasets", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	body := decodeBody[datasetListDTO](t, recorder)
	if len(body.Datasets) != 1 {
		t.Fatalf("datasets = %d, want 1", len(body.Datasets))
	}

	samples := body.Datasets[0]
	if samples.ID != "samples" {
		t.Errorf("id = %q, want samples", samples.ID)
	}
	if samples.TableCount != 2 {
		t.Errorf("tableCount = %d, want 2", samples.TableCount)
	}
	if samples.RowCount != 12 {
		t.Errorf("rowCount = %d, want 12", samples.RowCount)
	}
	if samples.Source.Filename != "built-in" {
		t.Errorf("source.filename = %q, want built-in", samples.Source.Filename)
	}
}

func TestPreviewDatasetInfersTypesWithoutCommitting(t *testing.T) {
	handler := newTestHandler()

	recorder := doMultipart(t, handler, "/api/v1/datasets/preview", nil, "people.csv",
		[]byte("id,name,active\n1,Ada,true\n2,Grace,false\n"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	preview := decodeBody[previewDTO](t, recorder)
	if preview.TableName != "people" {
		t.Errorf("tableName = %q, want people", preview.TableName)
	}
	if preview.RowCount != 2 {
		t.Errorf("rowCount = %d, want 2", preview.RowCount)
	}
	if len(preview.Columns) != 3 {
		t.Fatalf("columns = %d, want 3", len(preview.Columns))
	}
	if preview.Columns[0].Type != "integer" || preview.Columns[1].Type != "text" || preview.Columns[2].Type != "boolean" {
		t.Errorf("types = %v, want integer, text, boolean", []string{
			preview.Columns[0].Type, preview.Columns[1].Type, preview.Columns[2].Type})
	}
	if len(preview.Rows) != 2 {
		t.Fatalf("preview rows = %d, want 2", len(preview.Rows))
	}
	if preview.Rows[0][0] != "1" {
		t.Errorf("first cell = %#v, want the string \"1\"", preview.Rows[0][0])
	}
	if preview.Rows[0][2] != true {
		t.Errorf("boolean cell = %#v, want true", preview.Rows[0][2])
	}
	if preview.Warnings == nil {
		t.Error("warnings is null; want an empty array")
	}

	datasets := decodeBody[datasetListDTO](t, doRequest(t, handler, http.MethodGet, "/api/v1/datasets", nil))
	if len(datasets.Datasets) != 1 {
		t.Errorf("preview registered a dataset: %v", datasets.Datasets)
	}
}

func TestCreateDatasetThenBrowseTablesAndRows(t *testing.T) {
	handler := newTestHandler()

	created := doMultipart(t, handler, "/api/v1/datasets", map[string]string{
		"datasetName": "People Import",
	}, "people.csv", []byte("id,name\n1,Ada\n2,Grace\n"))
	if created.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", created.Code, created.Body.String())
	}

	detail := decodeBody[datasetDetailDTO](t, created)
	if detail.ID != "people-import" {
		t.Errorf("id = %q, want people-import", detail.ID)
	}
	if detail.Name != "People Import" {
		t.Errorf("name = %q, want People Import", detail.Name)
	}
	if detail.TableCount != 1 || len(detail.Tables) != 1 {
		t.Fatalf("tables = %+v, want one table", detail.Tables)
	}

	fetched := doRequest(t, handler, http.MethodGet, "/api/v1/datasets/people-import", nil)
	if fetched.Code != http.StatusOK {
		t.Fatalf("GET dataset status = %d, want 200", fetched.Code)
	}

	tables := decodeBody[tableListDTO](t, doRequest(t, handler, http.MethodGet, "/api/v1/datasets/people-import/tables", nil))
	if len(tables.Tables) != 1 || tables.Tables[0].ID != "people" {
		t.Fatalf("tables = %+v, want people", tables.Tables)
	}

	table := decodeBody[tableDetailDTO](t, doRequest(t, handler, http.MethodGet, "/api/v1/datasets/people-import/tables/people", nil))
	if len(table.Columns) != 2 {
		t.Fatalf("columns = %d, want 2", len(table.Columns))
	}
	if table.Columns[0].Type != "integer" {
		t.Errorf("id type = %q, want integer", table.Columns[0].Type)
	}

	rows := decodeBody[rowsPageDTO](t, doRequest(t, handler, http.MethodGet,
		"/api/v1/datasets/people-import/tables/people/rows?offset=1&limit=1", nil))
	if rows.TotalRows != 2 {
		t.Errorf("totalRows = %d, want 2", rows.TotalRows)
	}
	if rows.Offset != 1 || len(rows.Rows) != 1 {
		t.Errorf("offset = %d, rows = %d; want offset 1 and one row", rows.Offset, len(rows.Rows))
	}
	if rows.HasMore {
		t.Error("hasMore = true on the last page")
	}
}

func TestRowsSerializeNullsAndNumbersDistinctly(t *testing.T) {
	handler := newTestHandler()

	created := doMultipart(t, handler, "/api/v1/datasets", nil, "notes.csv",
		[]byte("id,note\n1,\n2,42\n"))
	if created.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", created.Code, created.Body.String())
	}

	rows := decodeBody[rowsPageDTO](t, doRequest(t, handler, http.MethodGet,
		"/api/v1/datasets/notes/tables/notes/rows", nil))

	if rows.Rows[0][1] != nil {
		t.Errorf("empty cell = %#v, want JSON null", rows.Rows[0][1])
	}
	if rows.Rows[1][1] != "42" {
		t.Errorf("integer cell = %#v, want the string \"42\"", rows.Rows[1][1])
	}
}

func TestDeleteDataset(t *testing.T) {
	handler := newTestHandler()

	recorder := doRequest(t, handler, http.MethodDelete, "/api/v1/datasets/samples", nil)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	if recorder.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", recorder.Body.String())
	}

	missing := doRequest(t, handler, http.MethodGet, "/api/v1/datasets/samples", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", missing.Code)
	}
	if got := decodeError(t, missing).Error.Code; got != "dataset_not_found" {
		t.Errorf("code = %q, want dataset_not_found", got)
	}
}

func TestPreviewReportsInvalidCSVWithLocation(t *testing.T) {
	recorder := doMultipart(t, newTestHandler(), "/api/v1/datasets/preview", nil, "broken.csv",
		[]byte("a,b\n1\n"))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}

	body := decodeError(t, recorder)
	if body.Error.Code != "invalid_csv" {
		t.Errorf("code = %q, want invalid_csv", body.Error.Code)
	}
	if body.Error.Location == nil {
		t.Fatal("location is missing")
	}
	if body.Error.Location.Line != 2 {
		t.Errorf("line = %d, want 2", body.Error.Location.Line)
	}
	if !strings.Contains(body.Error.Message, "expected 2 fields") {
		t.Errorf("message = %q, want a field-count explanation", body.Error.Message)
	}
}

func TestImportRejectsOversizedFile(t *testing.T) {
	oversized := bytes.Repeat([]byte("a"), (5<<20)+1)

	recorder := doMultipart(t, newTestHandler(), "/api/v1/datasets/preview", nil, "big.csv", oversized)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", recorder.Code)
	}
	if got := decodeError(t, recorder).Error.Code; got != "file_too_large" {
		t.Errorf("code = %q, want file_too_large", got)
	}
}

func TestImportRequiresAFilePart(t *testing.T) {
	recorder := doMultipart(t, newTestHandler(), "/api/v1/datasets/preview", map[string]string{"tableName": "x"}, "", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if got := decodeError(t, recorder).Error.Code; got != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", got)
	}
}

func TestUnknownDatasetAndTable(t *testing.T) {
	handler := newTestHandler()

	dataset := doRequest(t, handler, http.MethodGet, "/api/v1/datasets/nope", nil)
	if dataset.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", dataset.Code)
	}
	if got := decodeError(t, dataset).Error.Code; got != "dataset_not_found" {
		t.Errorf("code = %q, want dataset_not_found", got)
	}

	table := doRequest(t, handler, http.MethodGet, "/api/v1/datasets/samples/tables/nope", nil)
	if table.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", table.Code)
	}
	if got := decodeError(t, table).Error.Code; got != "table_not_found" {
		t.Errorf("code = %q, want table_not_found", got)
	}
}

func TestRowsPaginationRejectsInvalidParameters(t *testing.T) {
	handler := newTestHandler()

	for _, query := range []string{"offset=-1", "offset=abc", "limit=0", "limit=-5"} {
		t.Run(query, func(t *testing.T) {
			recorder := doRequest(t, handler, http.MethodGet,
				"/api/v1/datasets/samples/tables/employees/rows?"+query, nil)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", recorder.Code)
			}
			if got := decodeError(t, recorder).Error.Code; got != "invalid_request" {
				t.Errorf("code = %q, want invalid_request", got)
			}
		})
	}
}

func TestRowsPaginationClampsLimit(t *testing.T) {
	recorder := doRequest(t, newTestHandler(), http.MethodGet,
		"/api/v1/datasets/samples/tables/employees/rows?limit=999", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	rows := decodeBody[rowsPageDTO](t, recorder)
	if rows.Limit != maxRowLimit {
		t.Errorf("limit = %d, want %d", rows.Limit, maxRowLimit)
	}
	if len(rows.Rows) != 8 {
		t.Errorf("rows = %d, want all 8 sample employees", len(rows.Rows))
	}
}

func TestDatasetRoutesRejectUnsupportedMethods(t *testing.T) {
	recorder := doRequest(t, newTestHandler(), http.MethodPatch, "/api/v1/datasets", nil)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
	if got := recorder.Header().Get("Allow"); got != "GET, POST" {
		t.Errorf("Allow = %q, want GET, POST", got)
	}
}

func TestPreviewLiteralRouteWinsOverDatasetID(t *testing.T) {
	handler := newTestHandler()

	// GET /datasets/preview must be a method error on the preview route, not a
	// lookup of a dataset named "preview".
	recorder := doRequest(t, handler, http.MethodGet, "/api/v1/datasets/preview", nil)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
	if got := recorder.Header().Get("Allow"); got != "POST" {
		t.Errorf("Allow = %q, want POST", got)
	}
}
