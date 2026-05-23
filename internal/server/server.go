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

	"go-doc-server/internal/search"
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

	return logRequests(mux)
}

func (s *Server) serveStyle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Write(styleCSS)
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
		Results  []resultView
		Total    uint64
		Page     int
		HasPrev  bool
		HasNext  bool
		PrevPage int
		NextPage int
	}{
		Query:    query,
		Results:  results,
		Total:    res.Total,
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

// logRequests logs one structured event per request with method, path,
// status and latency.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
		)
	})
}
