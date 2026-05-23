package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intincrab/GoDocHive/internal/index"
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

	res, err := s.Search("apple", 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Documents) != 1 {
		t.Fatalf("got %d results, want 1", len(res.Documents))
	}
	if res.Total != 1 {
		t.Errorf("Total = %d, want 1", res.Total)
	}
	if res.Documents[0].Title != "Apple" {
		t.Errorf("title = %q, want %q", res.Documents[0].Title, "Apple")
	}
	if filepath.IsAbs(res.Documents[0].URL) {
		t.Errorf("URL = %q, want a path relative to root", res.Documents[0].URL)
	}
	if !strings.Contains(res.Documents[0].Snippet, "<mark>") {
		t.Errorf("Snippet = %q, want it to contain a <mark> highlight", res.Documents[0].Snippet)
	}
}

func TestSearchPaginates(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.html", "b.html", "c.html"} {
		body := `<html><head><title>` + name + `</title></head><body>common term</body></html>`
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := index.Open(filepath.Join(dir, "idx.bleve"), dir, index.DefaultExtensions, true)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	s := &Searcher{Index: idx, Root: dir}

	first, err := s.Search("common", 1, 2)
	if err != nil {
		t.Fatalf("Search page 1: %v", err)
	}
	if first.Total != 3 {
		t.Errorf("Total = %d, want 3", first.Total)
	}
	if len(first.Documents) != 2 {
		t.Errorf("page 1 returned %d docs, want 2", len(first.Documents))
	}

	second, err := s.Search("common", 2, 2)
	if err != nil {
		t.Fatalf("Search page 2: %v", err)
	}
	if len(second.Documents) != 1 {
		t.Errorf("page 2 returned %d docs, want 1", len(second.Documents))
	}
}

func TestSearchEmptyQueryReturnsNothing(t *testing.T) {
	s := newTestSearcher(t)

	res, err := s.Search("", 1, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Documents) != 0 {
		t.Errorf("empty query returned %d results, want 0", len(res.Documents))
	}
}
