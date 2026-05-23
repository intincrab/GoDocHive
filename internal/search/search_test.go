package search

import (
	"os"
	"path/filepath"
	"testing"

	"go-doc-server/internal/index"
)

func newTestSearcher(t *testing.T) *Searcher {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.html", `<html><head><title>Apple</title></head><body>apple pie recipe</body></html>`)
	write("b.html", `<html><head><title>Banana</title></head><body>banana bread</body></html>`)

	idx, err := index.Open(filepath.Join(dir, "idx.bleve"), dir, index.DefaultExtensions, true)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return &Searcher{Index: idx, Root: dir}
}

func TestSearchMatchesOneDoc(t *testing.T) {
	s := newTestSearcher(t)

	res, err := s.Search("apple")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("got %d results, want 1", len(res))
	}
	if res[0].Title != "Apple" {
		t.Errorf("title = %q, want %q", res[0].Title, "Apple")
	}
	if filepath.IsAbs(res[0].URL) {
		t.Errorf("URL = %q, want a path relative to root", res[0].URL)
	}
}

func TestSearchEmptyQueryReturnsNothing(t *testing.T) {
	s := newTestSearcher(t)

	res, err := s.Search("")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("empty query returned %d results, want 0", len(res))
	}
}
