package index

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestIndexFileAndDeleteFile(t *testing.T) {
	dir := t.TempDir()
	idx, err := Open(filepath.Join(dir, "idx.bleve"), dir, DefaultExtensions, true)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer idx.Close()

	p := filepath.Join(dir, "new.html")
	writeFile(t, p, `<html><head><title>New</title></head><body>fresh content</body></html>`)

	if err := IndexFile(idx, p); err != nil {
		t.Fatalf("IndexFile: %v", err)
	}
	if count, _ := idx.DocCount(); count != 1 {
		t.Fatalf("after IndexFile DocCount = %d, want 1", count)
	}

	if err := DeleteFile(idx, p); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if count, _ := idx.DocCount(); count != 0 {
		t.Fatalf("after DeleteFile DocCount = %d, want 0", count)
	}
}

func TestWatcherReindexesNewFile(t *testing.T) {
	dir := t.TempDir()
	idxPath := filepath.Join(dir, "idx.bleve")
	idx, err := Open(idxPath, dir, DefaultExtensions, true)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer idx.Close()

	w, err := NewWatcher(idx, dir, DefaultExtensions, idxPath)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	w.debounce = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	time.Sleep(100 * time.Millisecond) // let the watcher start

	writeFile(t, filepath.Join(dir, "live.html"),
		`<html><head><title>Live</title></head><body>live body</body></html>`)

	deadline := time.Now().Add(5 * time.Second)
	for {
		if count, _ := idx.DocCount(); count == 1 {
			return
		}
		if time.Now().After(deadline) {
			count, _ := idx.DocCount()
			t.Fatalf("new file was not indexed within timeout (DocCount=%d)", count)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
