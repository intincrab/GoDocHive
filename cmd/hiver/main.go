package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go-doc-server/internal/index"
	"go-doc-server/internal/search"
	"go-doc-server/internal/server"
)

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

// parseExtensions normalizes a comma-separated extension list, ensuring each
// entry is trimmed and dot-prefixed.
func parseExtensions(s string) []string {
	parts := strings.Split(s, ",")
	exts := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, ".") {
			p = "." + p
		}
		exts = append(exts, p)
	}
	return exts
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

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
	watch := flag.Bool("watch", true, "watch the path and re-index changes live")

	flag.Parse()

	root := *path
	exts := index.DefaultExtensions
	if *extensions != "" {
		exts = parseExtensions(*extensions)
	}

	slog.Info("starting",
		"path", root,
		"index", *indexPath,
		"refresh", *refresh,
		"extensions", exts,
	)

	idx, err := index.Open(*indexPath, root, exts, *refresh)
	if err != nil {
		fatal("opening index", err)
	}
	defer idx.Close()

	searcher := &search.Searcher{Index: idx, Root: root}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Keep the index in sync with on-disk changes. The watcher stops when ctx
	// is cancelled during shutdown.
	if *watch {
		watcher, err := index.NewWatcher(idx, root, exts, *indexPath)
		if err != nil {
			slog.Warn("live re-indexing disabled", "err", err)
		} else {
			go watcher.Run(ctx)
			slog.Info("live re-indexing enabled")
		}
	}

	// Explicit timeouts so a slow or idle client cannot hold a connection open
	// indefinitely (Slowloris). ReadHeaderTimeout in particular bounds the
	// header-read phase; WriteTimeout is generous to allow large doc responses.
	srv := &http.Server{
		Addr:              *addr,
		Handler:           server.New(root, searcher),
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
