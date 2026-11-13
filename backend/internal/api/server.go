package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/NiranjanS8/sql-execution-visualizer/backend/internal/config"
)

const serviceName = "querylens"

// Version is the build version reported by the health endpoint. Override it at
// build time with -ldflags "-X .../internal/api.Version=<version>".
var Version = "0.1.0-dev"

// Server owns the HTTP surface. It holds no execution state yet; later phases
// inject the catalog and query service here rather than reaching for globals.
type Server struct {
	cfg     config.Config
	log     *slog.Logger
	started time.Time
}

func NewServer(cfg config.Config, log *slog.Logger) *Server {
	return &Server{cfg: cfg, log: log, started: time.Now()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	s.route(mux, http.MethodGet, "/api/v1/health", s.handleHealth)

	mux.HandleFunc("/", s.handleNotFound)

	var handler http.Handler = mux
	handler = s.logRequests(handler)
	handler = s.recoverPanics(handler)
	handler = s.withCORS(handler)
	return handler
}

// route registers a handler for a method and path, plus a path-level fallback so
// a request with an unsupported method returns 405 rather than 404.
func (s *Server) route(mux *http.ServeMux, method, path string, handler http.HandlerFunc) {
	mux.HandleFunc(method+" "+path, handler)
	mux.HandleFunc(path, s.methodNotAllowed(method))
}
