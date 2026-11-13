package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/config"
)

func newTestHandler(origins ...string) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		Addr:           config.DefaultAddr,
		AllowedOrigins: origins,
		ShutdownGrace:  config.DefaultShutdownGrace,
	}
	return NewServer(cfg, logger).Handler()
}

func doRequest(t *testing.T, handler http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestHealthEndpoint(t *testing.T) {
	recorder := doRequest(t, newTestHandler(), http.MethodGet, "/api/v1/health", nil)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("content type = %q, want JSON", got)
	}

	var body healthResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Status, "ok")
	}
	if body.Service != serviceName {
		t.Errorf("service = %q, want %q", body.Service, serviceName)
	}
	if body.Version == "" {
		t.Error("version is empty")
	}
	if body.Time.IsZero() {
		t.Error("time is zero")
	}
	if body.UptimeMS < 0 {
		t.Errorf("uptimeMs = %d, want >= 0", body.UptimeMS)
	}
}

func TestUnknownRouteReturnsJSONNotFound(t *testing.T) {
	recorder := doRequest(t, newTestHandler(), http.MethodGet, "/api/v1/nope", nil)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	var body errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("code = %q, want %q", body.Error.Code, "not_found")
	}
	if body.Error.Message == "" {
		t.Error("message is empty")
	}
}

func TestUnsupportedMethodReturnsMethodNotAllowed(t *testing.T) {
	recorder := doRequest(t, newTestHandler(), http.MethodPost, "/api/v1/health", nil)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if got := recorder.Header().Get("Allow"); got != http.MethodGet {
		t.Errorf("Allow = %q, want %q", got, http.MethodGet)
	}

	var body errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "method_not_allowed" {
		t.Errorf("code = %q, want %q", body.Error.Code, "method_not_allowed")
	}
}

func TestCORSPreflightForAllowedOrigin(t *testing.T) {
	handler := newTestHandler("http://localhost:5173")
	recorder := doRequest(t, handler, http.MethodOptions, "/api/v1/health", map[string]string{
		"Origin": "http://localhost:5173",
	})

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("allow origin = %q, want the request origin", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("allow methods header is empty")
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	handler := newTestHandler("http://localhost:5173")
	recorder := doRequest(t, handler, http.MethodGet, "/api/v1/health", map[string]string{
		"Origin": "https://example.com",
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow origin = %q, want no header for a disallowed origin", got)
	}
}

func TestHealthDoesNotExposeServerDetails(t *testing.T) {
	recorder := doRequest(t, newTestHandler(), http.MethodGet, "/api/v1/health", nil)

	var raw map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	for _, forbidden := range []string{"goVersion", "os", "hostname", "config"} {
		if _, ok := raw[forbidden]; ok {
			t.Errorf("health response exposes %q", forbidden)
		}
	}
	if body, ok := raw["time"].(string); ok {
		if _, err := time.Parse(time.RFC3339Nano, body); err != nil {
			t.Errorf("time %q is not RFC3339: %v", body, err)
		}
	}
}
