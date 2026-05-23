package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractTitleAndContent(t *testing.T) {
	const doc = `<html><head><title>Hello</title></head><body>world foo</body></html>`
	title, content := extractTitleAndContent(doc)
	if title != "Hello" {
		t.Errorf("title = %q, want %q", title, "Hello")
	}
	if !strings.Contains(content, "world") || !strings.Contains(content, "foo") {
		t.Errorf("content = %q, want it to contain body text", content)
	}
}

func TestExtractTitleAndContentNoTitle(t *testing.T) {
	title, _ := extractTitleAndContent(`<html><body>x</body></html>`)
	if title != "" {
		t.Errorf("title = %q, want empty so caller can fall back to filename", title)
	}
}

func TestHasAllowedExtension(t *testing.T) {
	exts := []string{".html", ".md"}
	cases := []struct {
		name string
		want bool
	}{
		{"a.html", true},
		{"a.md", true},
		{"a.txt", false},
		{"a.HTML", false}, // case-sensitive by design
		{"noext", false},
	}
	for _, c := range cases {
		if got := hasAllowedExtension(c.name, exts); got != c.want {
			t.Errorf("hasAllowedExtension(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestOpenBuildsIndex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.html"), `<html><head><title>A</title></head><body>apple</body></html>`)
	writeFile(t, filepath.Join(dir, "b.txt"), "banana")
	writeFile(t, filepath.Join(dir, "skip.log"), "ignored")

	idx, err := Open(filepath.Join(dir, "idx.bleve"), dir, DefaultExtensions, true)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer idx.Close()

	count, err := idx.DocCount()
	if err != nil {
		t.Fatalf("DocCount: %v", err)
	}
	if count != 2 {
		t.Errorf("DocCount = %d, want 2 (skip.log should be excluded)", count)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
