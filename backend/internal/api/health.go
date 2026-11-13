package api

import (
	"net/http"
	"time"
)

type healthResponse struct {
	Status   string    `json:"status"`
	Service  string    `json:"service"`
	Version  string    `json:"version"`
	Time     time.Time `json:"time"`
	UptimeMS int64     `json:"uptimeMs"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.log, http.StatusOK, healthResponse{
		Status:   "ok",
		Service:  serviceName,
		Version:  Version,
		Time:     time.Now().UTC(),
		UptimeMS: time.Since(s.started).Milliseconds(),
	})
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, s.log, http.StatusNotFound, "not_found", "No route matches "+r.URL.Path+".")
}

func (s *Server) methodNotAllowed(allowed string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allowed)
		writeError(w, s.log, http.StatusMethodNotAllowed, "method_not_allowed",
			r.Method+" is not supported for "+r.URL.Path+". Use "+allowed+".")
	}
}
