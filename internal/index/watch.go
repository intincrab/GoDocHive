package index

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/fsnotify/fsnotify"
)

// IndexFile re-indexes a single file into idx, replacing any existing entry.
func IndexFile(idx bleve.Index, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	doc, err := readDocument(path, info.Name())
	if err != nil {
		return err
	}
	return idx.Index(path, doc)
}

// DeleteFile removes a file from idx. It is a no-op if the file is absent.
func DeleteFile(idx bleve.Index, path string) error {
	return idx.Delete(path)
}

// Watcher keeps a Bleve index in sync with a directory tree by watching for
// filesystem changes. Bleve indexes are safe for concurrent use, so updates
// happen in place while the server keeps searching the same index.
type Watcher struct {
	idx      bleve.Index
	root     string
	exts     []string
	exclude  string // cleaned absolute path of the index dir to ignore
	debounce time.Duration
	fsw      *fsnotify.Watcher
}

// NewWatcher creates a Watcher over root, ignoring everything under indexPath
// (the index's own files). The caller drives it with Run.
func NewWatcher(idx bleve.Index, root string, exts []string, indexPath string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	exclude, _ := filepath.Abs(indexPath)
	w := &Watcher{
		idx:      idx,
		root:     root,
		exts:     exts,
		exclude:  filepath.Clean(exclude),
		debounce: 300 * time.Millisecond,
		fsw:      fsw,
	}
	if err := w.addRecursive(root); err != nil {
		fsw.Close()
		return nil, err
	}
	return w, nil
}

// addRecursive registers a watch on dir and all of its subdirectories,
// skipping the excluded index directory.
func (w *Watcher) addRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if w.isExcluded(path) {
			return filepath.SkipDir
		}
		return w.fsw.Add(path)
	})
}

func (w *Watcher) isExcluded(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	abs = filepath.Clean(abs)
	return abs == w.exclude || strings.HasPrefix(abs, w.exclude+string(os.PathSeparator))
}

// Run processes filesystem events until ctx is cancelled, coalescing bursts of
// changes within the debounce window before applying them. It closes the
// underlying watcher on return.
func (w *Watcher) Run(ctx context.Context) {
	defer w.fsw.Close()

	pending := make(map[string]bool) // path -> exists (true=index, false=delete)
	var timer *time.Timer
	var fire <-chan time.Time

	schedule := func() {
		if timer == nil {
			timer = time.NewTimer(w.debounce)
			fire = timer.C
			return
		}
		timer.Reset(w.debounce)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if w.handleEvent(event, pending) {
				schedule()
			}
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			slog.Warn("watcher error", "err", err)
		case <-fire:
			w.flush(pending)
			pending = make(map[string]bool)
			timer = nil
			fire = nil
		}
	}
}

// handleEvent updates pending and reports whether a flush should be scheduled.
func (w *Watcher) handleEvent(event fsnotify.Event, pending map[string]bool) bool {
	if w.isExcluded(event.Name) {
		return false
	}

	// A newly created directory must be watched too so its files are tracked.
	if event.Op.Has(fsnotify.Create) {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := w.fsw.Add(event.Name); err != nil {
				slog.Warn("watch add failed", "path", event.Name, "err", err)
			}
			return false
		}
	}

	if !hasAllowedExtension(filepath.Base(event.Name), w.exts) {
		return false
	}

	switch {
	case event.Op.Has(fsnotify.Remove) || event.Op.Has(fsnotify.Rename):
		pending[event.Name] = false
	case event.Op.Has(fsnotify.Create) || event.Op.Has(fsnotify.Write):
		pending[event.Name] = true
	default:
		return false
	}
	return true
}

func (w *Watcher) flush(pending map[string]bool) {
	for path, exists := range pending {
		if exists {
			if err := IndexFile(w.idx, path); err != nil {
				slog.Warn("reindex failed", "path", path, "err", err)
				continue
			}
			slog.Info("reindexed", "path", path)
		} else {
			if err := DeleteFile(w.idx, path); err != nil {
				slog.Warn("deindex failed", "path", path, "err", err)
				continue
			}
			slog.Info("deindexed", "path", path)
		}
	}
}
