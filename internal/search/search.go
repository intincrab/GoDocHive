// Package search runs queries against a Bleve index and maps hits back to
// documents with URLs relative to the served root.
package search

import (
	"log/slog"
	"path/filepath"

	"github.com/blevesearch/bleve/v2"

	"go-doc-server/internal/index"
)

// Searcher executes queries against an index.
type Searcher struct {
	Index bleve.Index
	Root  string
}

// Search returns documents matching query. An empty query yields no results.
func (s *Searcher) Search(query string) ([]index.Document, error) {
	if query == "" {
		return nil, nil
	}

	req := bleve.NewSearchRequest(bleve.NewMatchQuery(query))
	req.Fields = []string{"Title", "Content", "URL"}
	req.Highlight = bleve.NewHighlight()

	res, err := s.Index.Search(req)
	if err != nil {
		return nil, err
	}

	var results []index.Document
	for _, hit := range res.Hits {
		relativeURL, err := filepath.Rel(s.Root, hit.Fields["URL"].(string))
		if err != nil {
			slog.Warn("skipping hit: cannot make relative URL", "url", hit.Fields["URL"], "err", err)
			continue
		}
		results = append(results, index.Document{
			Title:   hit.Fields["Title"].(string),
			Content: hit.Fields["Content"].(string),
			URL:     relativeURL,
		})
	}
	return results, nil
}
