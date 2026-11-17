package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/dataset"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/query"
)

// maxQueryRequestBytes leaves room for the SQL limit plus JSON overhead while
// keeping a malformed or hostile body bounded.
const maxQueryRequestBytes = 64 << 10

func (s *Server) handleRunQuery(w http.ResponseWriter, r *http.Request) {
	body, ok := s.readQueryRequest(w, r)
	if !ok {
		return
	}

	entry, found := s.datasets.Get(dataset.ID(body.DatasetID))
	if !found {
		s.writeDatasetNotFound(w, dataset.ID(body.DatasetID))
		return
	}

	result, err := s.queries.Execute(r.Context(), body.SQL, entry.Tables)
	if err != nil {
		s.writeQueryFailure(w, r, err)
		return
	}

	writeJSON(w, s.log, http.StatusOK, queryResponse(result, body.DatasetID, s.queries.Limits()))
}

func (s *Server) readQueryRequest(w http.ResponseWriter, r *http.Request) (queryRequestDTO, bool) {
	var body queryRequestDTO

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxQueryRequestBytes))
	if err := decoder.Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, s.log, http.StatusRequestEntityTooLarge, "invalid_request",
				fmt.Sprintf("The query request exceeds the %d KiB limit.", maxQueryRequestBytes>>10))
			return body, false
		}
		writeError(w, s.log, http.StatusBadRequest, "invalid_request",
			`Expected a JSON body with "datasetId" and "sql".`)
		return body, false
	}

	body.DatasetID = strings.TrimSpace(body.DatasetID)
	if body.DatasetID == "" {
		writeError(w, s.log, http.StatusBadRequest, "invalid_request",
			`"datasetId" is required.`)
		return body, false
	}

	return body, true
}

func (s *Server) writeQueryFailure(w http.ResponseWriter, r *http.Request, err error) {
	var queryErr *query.Error
	if !errors.As(err, &queryErr) {
		s.log.Error("query failed", "err", err)
		writeError(w, s.log, http.StatusInternalServerError, "internal_error",
			"The query could not be executed.")
		return
	}

	// A cancelled client is gone; writing a response would only fail again.
	if queryErr.Code == query.CodeCancelled && r.Context().Err() != nil {
		return
	}

	if cause := errors.Unwrap(queryErr); cause != nil {
		s.log.Error("query execution failed", "code", queryErr.Code, "err", cause)
	}

	writeErrorWithLocation(w, s.log, queryStatus(queryErr.Code),
		string(queryErr.Code), queryErr.Message, queryErr.Line, queryErr.Column)
}

func queryStatus(code query.Code) int {
	switch code {
	case query.CodeExecutionError, query.CodeResultLimit:
		return http.StatusUnprocessableEntity
	case query.CodeTimeout, query.CodeCancelled:
		return http.StatusRequestTimeout
	default:
		return http.StatusBadRequest
	}
}
