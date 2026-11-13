package api

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/config"
	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/dataset"
)

const serviceName = "querylens"

// Version is the build version reported by the health endpoint. Override it at
// build time with -ldflags "-X .../internal/api.Version=<version>".
var Version = "0.1.0-dev"

// Server owns the HTTP surface. Dependencies are injected rather than reached
// for as globals, so handlers stay testable.
type Server struct {
	cfg      config.Config
	log      *slog.Logger
	datasets *dataset.Store
	started  time.Time
}

func NewServer(cfg config.Config, log *slog.Logger, datasets *dataset.Store) *Server {
	return &Server{cfg: cfg, log: log, datasets: datasets, started: time.Now()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	s.route(mux, "/api/v1/health", methods{
		http.MethodGet: s.handleHealth,
	})
	s.route(mux, "/api/v1/datasets", methods{
		http.MethodGet:  s.handleListDatasets,
		http.MethodPost: s.handleCreateDataset,
	})
	s.route(mux, "/api/v1/datasets/preview", methods{
		http.MethodPost: s.handlePreviewDataset,
	})
	s.route(mux, "/api/v1/datasets/{datasetID}", methods{
		http.MethodGet:    s.handleGetDataset,
		http.MethodDelete: s.handleDeleteDataset,
	})
	s.route(mux, "/api/v1/datasets/{datasetID}/tables", methods{
		http.MethodGet: s.handleListTables,
	})
	s.route(mux, "/api/v1/datasets/{datasetID}/tables/{tableID}", methods{
		http.MethodGet: s.handleGetTable,
	})
	s.route(mux, "/api/v1/datasets/{datasetID}/tables/{tableID}/rows", methods{
		http.MethodGet: s.handleTableRows,
	})

	mux.HandleFunc("/", s.handleNotFound)

	var handler http.Handler = mux
	handler = s.logRequests(handler)
	handler = s.recoverPanics(handler)
	handler = s.withCORS(handler)
	return handler
}

// methods maps an HTTP method to its handler for one path.
type methods map[string]http.HandlerFunc

// route registers a path with a dispatcher that selects the handler for the
// request method. Unsupported methods get 405 with an Allow header, and keeping
// the pattern methodless lets literal paths such as /datasets/preview take
// precedence over /datasets/{datasetID}.
func (s *Server) route(mux *http.ServeMux, path string, handlers methods) {
	notAllowed := s.methodNotAllowed(handlers)
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if handler, ok := handlers[r.Method]; ok {
			handler(w, r)
			return
		}
		notAllowed(w, r)
	})
}

func (s *Server) methodNotAllowed(handlers methods) http.HandlerFunc {
	allowed := make([]string, 0, len(handlers))
	for method := range handlers {
		allowed = append(allowed, method)
	}
	slices.Sort(allowed)
	allow := strings.Join(allowed, ", ")

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allow)
		writeError(w, s.log, http.StatusMethodNotAllowed, "method_not_allowed",
			r.Method+" is not supported for "+r.URL.Path+". Use "+allow+".")
	}
}
