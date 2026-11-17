package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/config"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/dataset"
)

func newQueryTestHandler(t *testing.T, mutate func(*config.Config)) http.Handler {
	t.Helper()

	cfg := config.Config{
		QueryMaxSQLLength: config.DefaultQueryMaxSQLLength,
		QueryTimeout:      config.DefaultQueryTimeout,
		QueryMaxRows:      config.DefaultQueryMaxRows,
		QuerySampleLimit:  config.DefaultQuerySampleLimit,
	}
	if mutate != nil {
		mutate(&cfg)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	datasets := dataset.NewStore()
	if err := dataset.SeedSamples(datasets); err != nil {
		t.Fatalf("seed samples: %v", err)
	}
	return NewServer(cfg, logger, datasets).Handler()
}

func doJSON(t *testing.T, handler http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	return doRawJSON(t, handler, encoded)
}

func doRawJSON(t *testing.T, handler http.Handler, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	return doRawJSONWithContext(t, handler, context.Background(), body)
}

func doRawJSONWithContext(
	t *testing.T,
	handler http.Handler,
	ctx context.Context,
	body []byte,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/queries", bytes.NewReader(body)).
		WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestRunQueryReturnsRowsAndExecutionTrace(t *testing.T) {
	recorder := doJSON(t, newQueryTestHandler(t, nil), queryRequestDTO{
		DatasetID: "samples",
		SQL:       "SELECT name, salary FROM employees WHERE salary > 120000 ORDER BY salary DESC LIMIT 3",
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	body := decodeBody[queryResponseDTO](t, recorder)
	if body.QueryRunID == "" {
		t.Error("queryRunId is empty")
	}
	if body.Status != "completed" {
		t.Errorf("status = %q, want completed", body.Status)
	}
	if body.DatasetID != "samples" {
		t.Errorf("datasetId = %q, want samples", body.DatasetID)
	}
	if body.RowCount != 3 || len(body.Rows) != 3 {
		t.Fatalf("rowCount = %d, rows = %d; want 3", body.RowCount, len(body.Rows))
	}
	if body.ResultLimit != config.DefaultQueryMaxRows {
		t.Errorf("resultLimit = %d, want %d", body.ResultLimit, config.DefaultQueryMaxRows)
	}
	if body.DurationMS < 0 {
		t.Errorf("durationMs = %f, want non-negative", body.DurationMS)
	}

	if len(body.Columns) != 2 {
		t.Fatalf("columns = %d, want 2", len(body.Columns))
	}
	if body.Columns[0].Name != "name" || body.Columns[0].Type != "text" {
		t.Errorf("first column = %+v, want name text", body.Columns[0])
	}
	if body.Columns[1].Name != "salary" || body.Columns[1].Type != "integer" {
		t.Errorf("second column = %+v, want salary integer", body.Columns[1])
	}

	want := [][2]string{
		{"Alan Turing", "151000"},
		{"Ada Lovelace", "145000"},
		{"Grace Hopper", "132000"},
	}
	for i, pair := range want {
		name, ok := body.Rows[i][0].(string)
		if !ok || name != pair[0] {
			t.Errorf("row %d name = %#v, want %q", i, body.Rows[i][0], pair[0])
		}
		salary, ok := body.Rows[i][1].(string)
		if !ok || salary != pair[1] {
			t.Errorf("row %d salary = %#v, want the string %q", i, body.Rows[i][1], pair[1])
		}
	}

	if body.Execution.RootID != "n1" {
		t.Errorf("rootId = %q, want n1", body.Execution.RootID)
	}
	if body.Execution.Status != "completed" {
		t.Errorf("execution status = %q, want completed", body.Execution.Status)
	}
	if len(body.Execution.Nodes) != 5 {
		t.Fatalf("execution nodes = %d, want 5", len(body.Execution.Nodes))
	}
	if len(body.Execution.Edges) != 4 {
		t.Errorf("execution edges = %d, want 4", len(body.Execution.Edges))
	}
	if body.Execution.Nodes[0].Kind != "limit" {
		t.Errorf("root kind = %q, want limit", body.Execution.Nodes[0].Kind)
	}

	var project *executionNodeDTO
	for i := range body.Execution.Nodes {
		if body.Execution.Nodes[i].Kind == "project" {
			project = &body.Execution.Nodes[i]
			break
		}
	}
	if project == nil {
		t.Fatal("execution has no project node")
	}
	if len(project.Input.Columns) != 5 {
		t.Errorf("project input columns = %d, want the 5 source columns", len(project.Input.Columns))
	}
	if len(project.Output.Columns) != 2 {
		t.Errorf("project output columns = %d, want the 2 projected columns", len(project.Output.Columns))
	}
}

func TestRunQueryEncodesNullAndEmptyStringDistinctly(t *testing.T) {
	recorder := doJSON(t, newQueryTestHandler(t, nil), queryRequestDTO{
		DatasetID: "samples",
		SQL:       "SELECT NULL AS missing, '' AS empty, salary FROM employees LIMIT 1",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	body := decodeBody[queryResponseDTO](t, recorder)
	if len(body.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(body.Rows))
	}
	if body.Rows[0][0] != nil {
		t.Errorf("NULL cell = %#v, want JSON null", body.Rows[0][0])
	}
	if body.Rows[0][1] != "" {
		t.Errorf("empty string cell = %#v, want the empty string", body.Rows[0][1])
	}
	if body.Rows[0][2] != "145000" {
		t.Errorf("integer cell = %#v, want the string \"145000\"", body.Rows[0][2])
	}
}

func TestRunQueryReportsStructuredErrors(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantStatus int
		wantCode   string
	}{
		{name: "parse", sql: "SELECT FROM employees", wantStatus: http.StatusBadRequest, wantCode: "parse_error"},
		{name: "unknown table", sql: "SELECT * FROM nope", wantStatus: http.StatusBadRequest, wantCode: "unknown_table"},
		{name: "unknown column", sql: "SELECT nope FROM employees", wantStatus: http.StatusBadRequest, wantCode: "unknown_column"},
		{name: "type", sql: "SELECT name FROM employees WHERE salary", wantStatus: http.StatusBadRequest, wantCode: "type_error"},
		{name: "execution", sql: "SELECT salary / 0 FROM employees", wantStatus: http.StatusUnprocessableEntity, wantCode: "execution_error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := doJSON(t, newQueryTestHandler(t, nil), queryRequestDTO{
				DatasetID: "samples",
				SQL:       test.sql,
			})
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}

			body := decodeError(t, recorder)
			if body.Error.Code != test.wantCode {
				t.Errorf("code = %q, want %q", body.Error.Code, test.wantCode)
			}
			if body.Error.Message == "" {
				t.Error("message is empty")
			}
		})
	}
}

func TestRunQueryRejectsSQLOverTheLengthLimit(t *testing.T) {
	handler := newQueryTestHandler(t, func(cfg *config.Config) {
		cfg.QueryMaxSQLLength = 10
	})

	recorder := doJSON(t, handler, queryRequestDTO{
		DatasetID: "samples",
		SQL:       "SELECT * FROM employees",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
	if got := decodeError(t, recorder).Error.Code; got != "invalid_request" {
		t.Errorf("code = %q, want invalid_request", got)
	}
}

func TestRunQueryEnforcesResultLimit(t *testing.T) {
	handler := newQueryTestHandler(t, func(cfg *config.Config) {
		cfg.QueryMaxRows = 3
	})

	recorder := doJSON(t, handler, queryRequestDTO{
		DatasetID: "samples",
		SQL:       "SELECT * FROM employees",
	})
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422: %s", recorder.Code, recorder.Body.String())
	}

	body := decodeError(t, recorder)
	if body.Error.Code != "result_limit_exceeded" {
		t.Errorf("code = %q, want result_limit_exceeded", body.Error.Code)
	}
	if !strings.Contains(body.Error.Message, "3 rows") {
		t.Errorf("message = %q, want the configured row limit", body.Error.Message)
	}
}

func TestRunQueryReportsTimeout(t *testing.T) {
	handler := newQueryTestHandler(t, nil)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	recorder := doRawJSONWithContext(t, handler, ctx, []byte(
		`{"datasetId":"samples","sql":"SELECT * FROM employees"}`,
	))
	if recorder.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want 408: %s", recorder.Code, recorder.Body.String())
	}
	if got := decodeError(t, recorder).Error.Code; got != "timeout" {
		t.Errorf("code = %q, want timeout", got)
	}
}

func TestRunQuerySuppressesResponseWhenClientCancelled(t *testing.T) {
	handler := newQueryTestHandler(t, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	recorder := doRawJSONWithContext(t, handler, ctx, []byte(
		`{"datasetId":"samples","sql":"SELECT * FROM employees"}`,
	))
	if recorder.Body.Len() != 0 {
		t.Errorf("body = %q, want no response for a cancelled client", recorder.Body.String())
	}
}

func TestRunQueryRejectsInvalidRequests(t *testing.T) {
	handler := newQueryTestHandler(t, nil)

	t.Run("malformed JSON", func(t *testing.T) {
		recorder := doRawJSON(t, handler, []byte("{"))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", recorder.Code)
		}
		if got := decodeError(t, recorder).Error.Code; got != "invalid_request" {
			t.Errorf("code = %q, want invalid_request", got)
		}
	})

	t.Run("missing dataset", func(t *testing.T) {
		recorder := doJSON(t, handler, queryRequestDTO{SQL: "SELECT 1"})
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", recorder.Code)
		}
		if got := decodeError(t, recorder).Error.Code; got != "invalid_request" {
			t.Errorf("code = %q, want invalid_request", got)
		}
	})

	t.Run("unknown dataset", func(t *testing.T) {
		recorder := doJSON(t, handler, queryRequestDTO{DatasetID: "nope", SQL: "SELECT 1"})
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", recorder.Code)
		}
		if got := decodeError(t, recorder).Error.Code; got != "dataset_not_found" {
			t.Errorf("code = %q, want dataset_not_found", got)
		}
	})

	t.Run("oversized body", func(t *testing.T) {
		body := []byte(`{"datasetId":"samples","sql":"` +
			strings.Repeat("a", maxQueryRequestBytes+1) + `"}`)
		recorder := doRawJSON(t, handler, body)
		if recorder.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413", recorder.Code)
		}
		if got := decodeError(t, recorder).Error.Code; got != "invalid_request" {
			t.Errorf("code = %q, want invalid_request", got)
		}
	})
}

func TestRunQueryReturnsArraysForEmptyResults(t *testing.T) {
	recorder := doJSON(t, newQueryTestHandler(t, nil), queryRequestDTO{
		DatasetID: "samples",
		SQL:       "SELECT * FROM employees LIMIT 0",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := string(raw["rows"]); got != "[]" {
		t.Errorf("rows = %s, want []", got)
	}
}

func TestRunQueryRejectsUnsupportedMethods(t *testing.T) {
	recorder := doRequest(t, newQueryTestHandler(t, nil), http.MethodGet, "/api/v1/queries", nil)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
	if got := recorder.Header().Get("Allow"); got != http.MethodPost {
		t.Errorf("Allow = %q, want POST", got)
	}
}
