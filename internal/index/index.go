// Package index manages the on-disk Bleve index: opening/creating it and
// extracting indexable content from documents on disk.
package index

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/v2/mapping"
	"golang.org/x/net/html"
)

// DefaultExtensions are indexed when the caller does not specify a set.
var DefaultExtensions = []string{".html", ".htm", ".txt", ".md"}

// Document is a single indexed document.
type Document struct {
	Title   string
	Content string
	URL     string
}

// Open opens the Bleve index at path. If it does not exist it is created and
// built from root. When refresh is true any existing index is removed first so
// it is rebuilt from scratch. The caller owns Close on the returned index.
func Open(path, root string, exts []string, refresh bool) (bleve.Index, error) {
	if refresh {
		if _, err := os.Stat(path); err == nil {
			if err := os.RemoveAll(path); err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	idx, err := bleve.Open(path)
	switch {
	case err == bleve.ErrorIndexPathDoesNotExist:
		idx, err = bleve.New(path, newMapping())
		if err != nil {
			return nil, err
		}
		if err := Build(idx, root, exts); err != nil {
			idx.Close()
			return nil, err
		}
		return idx, nil
	case err != nil:
		return nil, err
	default:
		return idx, nil
	}
}

func newMapping() mapping.IndexMapping {
	indexMapping := bleve.NewIndexMapping()
	documentMapping := bleve.NewDocumentMapping()

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Analyzer = standard.Name

	documentMapping.AddFieldMappingsAt("Title", textFieldMapping)
	documentMapping.AddFieldMappingsAt("Content", textFieldMapping)
	documentMapping.AddFieldMappingsAt("URL", textFieldMapping)

	indexMapping.AddDocumentMapping("document", documentMapping)
	return indexMapping
}

// Build walks root and indexes every file whose name ends in an allowed
// extension into idx.
func Build(idx bleve.Index, root string, exts []string) error {
	batch := idx.NewBatch()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !hasAllowedExtension(info.Name(), exts) {
			return nil
		}
		doc, err := readDocument(path, info.Name())
		if err != nil {
			return err
		}
		return batch.Index(path, doc)
	})
	if err != nil {
		return err
	}
	return idx.Batch(batch)
}

// readDocument reads and parses a single file into a Document.
func readDocument(path, name string) (Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	title, body := extractTitleAndContent(string(content))
	if title == "" {
		title = name
	}
	return Document{Title: title, Content: body, URL: path}, nil
}

func hasAllowedExtension(filename string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
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
