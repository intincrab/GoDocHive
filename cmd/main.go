package main

import (
	"context"
	_ "embed"
	"flag"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"golang.org/x/net/html"
)

// Document is
type Document struct {
	Title   string
	Content string
	URL     string
}

// List of allowed file extensions
var allowedExtensions = []string{".html", ".htm", ".txt", ".md"}

var index bleve.Index

var root string

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

// envOr returns the environment value for key, or fallback when unset/empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// defaultAddr resolves the default listen address, honoring PORT when set.
// An explicit ADDR env var or -addr flag takes precedence over this.
func defaultAddr() string {
	if p := os.Getenv("PORT"); p != "" {
		return "127.0.0.1:" + p
	}
	return "127.0.0.1:3030"
}

// fatal logs a structured error and exits non-zero. Used only for
// unrecoverable startup failures, never inside request handlers.
func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
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

func main() {
	var err error

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// current working directory where the binary is run
	currentDir, err := os.Getwd()
	if err != nil {
		fatal("getting current working directory", err)
	}

	path := flag.String("path", currentDir, "Path to the directory")
	refresh := flag.Bool("refresh", false, "refresh/rebuild the index")
	extensions := flag.String("extensions", "", "Comma-separated list of file extensions to include")
	// Bind to loopback by default: this server has no auth and serves arbitrary
	// on-disk docs, so it must not be reachable off-host unless deliberately
	// placed behind a reverse proxy. Pass -addr 0.0.0.0:PORT to expose it.
	addr := flag.String("addr", envOr("ADDR", defaultAddr()), "address to listen on (host:port)")
	indexPath := flag.String("index", envOr("INDEX_PATH", "index.bleve"), "path to the bleve index directory")

	flag.Parse()

	root = *path
	if *extensions != "" {
		allowedExtensions = strings.Split(*extensions, ",")
		for i, ext := range allowedExtensions {
			allowedExtensions[i] = strings.TrimSpace(ext)
			if !strings.HasPrefix(allowedExtensions[i], ".") {
				allowedExtensions[i] = "." + allowedExtensions[i]
			}
		}
	}

	slog.Info("starting",
		"path", *path,
		"index", *indexPath,
		"refresh", *refresh,
		"extensions", allowedExtensions,
	)

	if *refresh {
		if _, err := os.Stat(*indexPath); err == nil {

			err = os.RemoveAll(*indexPath)
			if err != nil {
				fatal("deleting existing index", err)
			}
		} else if !os.IsNotExist(err) {
			fatal("checking index path", err)
		}

	}

	index, err = bleve.Open(*indexPath)
	if err == bleve.ErrorIndexPathDoesNotExist {

		indexMapping := bleve.NewIndexMapping()
		documentMapping := bleve.NewDocumentMapping()

		textFieldMapping := bleve.NewTextFieldMapping()
		textFieldMapping.Analyzer = standard.Name

		documentMapping.AddFieldMappingsAt("Title", textFieldMapping)
		documentMapping.AddFieldMappingsAt("Content", textFieldMapping)
		documentMapping.AddFieldMappingsAt("URL", textFieldMapping)

		indexMapping.AddDocumentMapping("document", documentMapping)

		index, err = bleve.New(*indexPath, indexMapping)
		if err != nil {
			fatal("creating index", err)
		}
		buildIndex(root)
	} else if err != nil {
		fatal("opening index", err)
	}
	defer index.Close()

	// http.Dir + FileServer rejects ".." and absolute-path escapes that the old
	// filepath.Join(root, r.URL.Path) pattern did not guard. Caveat: http.Dir
	// still follows symlinks, so a symlink inside root pointing outside root
	// would be served. Keep the served tree free of escaping symlinks (or run
	// behind a trust boundary) since we serve arbitrary on-disk docs.
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(root)))
	mux.HandleFunc("/search", handleSearch)
	mux.HandleFunc("/_assets/style.css", serveStyle)

	// Explicit timeouts so a slow or idle client cannot hold a connection open
	// indefinitely (Slowloris). ReadHeaderTimeout in particular bounds the
	// header-read phase; WriteTimeout is generous to allow large doc responses.
	srv := &http.Server{
		Addr:              *addr,
		Handler:           logRequests(mux),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Run the listener in the background so main can wait on either a fatal
	// serve error or an OS signal, then shut down cleanly.
	serveErr := make(chan error, 1)
	go func() {
		slog.Info("server listening", "addr", *addr)
		serveErr <- srv.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			fatal("server error", err)
		}
	case <-ctx.Done():
		stop() // restore default signal handling for a second Ctrl+C
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "err", err)
		}
	}
}

func hasAllowedExtension(filename string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
}

func buildIndex(root string) {
	batch := index.NewBatch()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && hasAllowedExtension(info.Name(), allowedExtensions) {
			// if !info.IsDir() && strings.HasSuffix(info.Name(), ".html") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			title, bodyContent := extractTitleAndContent(string(content))
			if title == "" {
				title = info.Name()
			}

			doc := Document{
				Title:   title,
				Content: bodyContent,
				URL:     path,
			}

			err = batch.Index(path, doc)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		fatal("building index", err)
	}

	err = index.Batch(batch)
	if err != nil {
		fatal("indexing batch", err)
	}
}

func extractTitleAndContent(content string) (string, string) {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		return "", ""
	}

	var title string
	var bodyContent strings.Builder

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "title" && n.FirstChild != nil {
				title = n.FirstChild.Data
			} else if n.Data == "body" {
				extractText(n, &bodyContent)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(doc)
	return title, bodyContent.String()
}

func extractText(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		sb.WriteString(" ")
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, sb)
	}
}

func serveStyle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Write(styleCSS)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	results, err := performSearch(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Query   string
		Results []Document
	}{
		Query:   query,
		Results: results,
	}

	if err := searchTmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func performSearch(query string) ([]Document, error) {
	var results []Document

	if query != "" {
		searchQuery := bleve.NewMatchQuery(query)
		searchRequest := bleve.NewSearchRequest(searchQuery)
		searchRequest.Fields = []string{"Title", "Content", "URL"}
		searchRequest.Highlight = bleve.NewHighlight()
		searchResult, err := index.Search(searchRequest)
		if err != nil {
			return nil, err
		}

		for _, hit := range searchResult.Hits {
			relativeURL, err := filepath.Rel(root, hit.Fields["URL"].(string))
			if err != nil {
				slog.Warn("skipping hit: cannot make relative URL", "url", hit.Fields["URL"], "err", err)
				continue
			}
			doc := Document{
				Title:   hit.Fields["Title"].(string),
				Content: hit.Fields["Content"].(string),
				URL:     relativeURL,
			}
			results = append(results, doc)
		}
	}

	return results, nil
}
