// Package server wires the HTTP handlers: static file serving, the search
// page, and request logging.
package server

import (
	_ "embed"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/intincrab/GoDocHive/internal/search"
)

//go:embed templates/search.html
var searchHTML string

//go:embed static/style.css
var styleCSS []byte

// Parsed once at startup rather than per request.
var searchTmpl = template.Must(template.New("search").Funcs(template.FuncMap{
	"truncate": func(s string, l int) string {
		if len(s) > l {
			return s[:l] + "..."
		}
		return s
	},
}).Parse(searchHTML))

// Server holds the dependencies shared by the HTTP handlers.
type Server struct {
	root     string
	searcher *search.Searcher
}

// New builds an http.Handler that serves files from root, exposes /search
// backed by searcher, and logs every request.
func New(root string, searcher *search.Searcher) http.Handler {
	s := &Server{root: root, searcher: searcher}

	mux := http.NewServeMux()
	// http.Dir + FileServer rejects ".." and absolute-path escapes that the old
	// filepath.Join(root, r.URL.Path) pattern did not guard. Caveat: http.Dir
	// still follows symlinks, so a symlink inside root pointing outside root
	// would be served. Keep the served tree free of escaping symlinks (or run
	// behind a trust boundary) since we serve arbitrary on-disk docs.
	mux.Handle("/", http.FileServer(http.Dir(root)))
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/_assets/style.css", s.serveStyle)
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.Handle("/metrics", promhttp.Handler())

	return logRequests(mux)
}

func (s *Server) serveStyle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Write(styleCSS)
}

// handleHealthz is a liveness probe: it returns 200 as long as the process is
// serving requests.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("ok"))
}

// handleReadyz is a readiness probe: it returns 200 only when the index is
// responsive, and 503 otherwise so load balancers stop routing traffic.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.searcher.Ready(); err != nil {
		slog.Warn("readiness check failed", "err", err)
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("ready"))
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}

	res, err := s.searcher.Search(query, page, search.DefaultPageSize)
	if err != nil {
		slog.Error("search failed", "query", query, "err", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	results := make([]resultView, 0, len(res.Documents))
	for _, hit := range res.Documents {
		results = append(results, resultView{
			Title: hit.Title,
			URL:   hit.URL,
			// Snippet is produced by Bleve's html highlighter (escaped text
			// with <mark> tags), so it is safe to mark as trusted HTML.
			Snippet: template.HTML(hit.Snippet), //nolint:gosec // see comment
			Content: hit.Content,
		})
	}

	data := struct {
		Query    string
		Searched bool
		Results  []resultView
		Total    int
		Page     int
		HasPrev  bool
		HasNext  bool
		PrevPage int
		NextPage int
	}{
		Query:    query,
		Searched: query != "",
		Results:  results,
		Total:    int(res.Total),
		Page:     res.Page,
		HasPrev:  res.Page > 1,
		HasNext:  uint64(res.Page*res.PageSize) < res.Total,
		PrevPage: res.Page - 1,
		NextPage: res.Page + 1,
	}

	if err := searchTmpl.Execute(w, data); err != nil {
		slog.Error("rendering search page", "err", err)
	}
}

// resultView is the per-hit shape passed to the template. Snippet is trusted
// HTML from the highlighter; Content is the plain-text fallback.
type resultView struct {
	Title   string
	URL     string
	Content string
	Snippet template.HTML
}

// statusRecorder captures the response status code for request logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// logRequests records Prometheus metrics for every request and logs one
// structured event per request (method, path, status, latency), skipping the
// ops endpoints to avoid flooding the log with health and scrape traffic.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start)

		observe(r.Method, rec.status, elapsed.Seconds())

		if !isQuietPath(r.URL.Path) {
			slog.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration", elapsed.String(),
				"remote", r.RemoteAddr,
			)
		}
	})
}

// isQuietPath reports whether a path is an ops endpoint that should not be
// logged on every hit.
func isQuietPath(p string) bool {
	switch p {
	case "/healthz", "/readyz", "/metrics":
		return true
	default:
		return false
	}
}
