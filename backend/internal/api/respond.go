package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type errorLocation struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

type errorBody struct {
	Code     string         `json:"code"`
	Message  string         `json:"message"`
	Location *errorLocation `json:"location,omitempty"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

func writeJSON(w http.ResponseWriter, log *slog.Logger, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Error("failed to encode response", "err", err)
	}
}

func writeError(w http.ResponseWriter, log *slog.Logger, status int, code, message string) {
	writeJSON(w, log, status, errorEnvelope{Error: errorBody{Code: code, Message: message}})
}

// writeErrorWithLocation reports an error at a source position. A zero line or
// column is omitted from the response.
func writeErrorWithLocation(w http.ResponseWriter, log *slog.Logger, status int, code, message string, line, column int) {
	body := errorBody{Code: code, Message: message}
	if line > 0 || column > 0 {
		body.Location = &errorLocation{Line: line, Column: column}
	}
	writeJSON(w, log, status, errorEnvelope{Error: body})
}
