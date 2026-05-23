// Package search runs queries against a Bleve index and maps hits back to
// documents with URLs relative to the served root.
package search

import (
	"log/slog"
	"path/filepath"

	"github.com/blevesearch/bleve/v2"

	"go-doc-server/internal/index"
)

// DefaultPageSize is the number of hits returned per page when the caller
// does not specify a size.
const DefaultPageSize = 10

// Searcher executes queries against an index.
type Searcher struct {
	Index bleve.Index
	Root  string
}

// Result is a single page of search hits plus the paging metadata needed to
// render navigation.
type Result struct {
	Documents []index.Document
	Total     uint64
	Page      int
	PageSize  int
}

// Search returns the requested page of documents matching query. page is
// 1-based; an empty query yields no results. Non-positive page/size values
// fall back to page 1 and DefaultPageSize.
func (s *Searcher) Search(query string, page, size int) (Result, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = DefaultPageSize
	}
	if query == "" {
		return Result{Page: page, PageSize: size}, nil
	}

	req := bleve.NewSearchRequest(bleve.NewMatchQuery(query))
	req.Fields = []string{"Title", "Content", "URL"}
	req.Highlight = bleve.NewHighlight()
	req.Size = size
	req.From = (page - 1) * size

	res, err := s.Index.Search(req)
	if err != nil {
		return Result{}, err
	}

	documents := make([]index.Document, 0, len(res.Hits))
	for _, hit := range res.Hits {
		relativeURL, err := filepath.Rel(s.Root, hit.Fields["URL"].(string))
		if err != nil {
			slog.Warn("skipping hit: cannot make relative URL", "url", hit.Fields["URL"], "err", err)
			continue
		}
		documents = append(documents, index.Document{
			Title:   hit.Fields["Title"].(string),
			Content: hit.Fields["Content"].(string),
			URL:     relativeURL,
		})
	}

	return Result{Documents: documents, Total: res.Total, Page: page, PageSize: size}, nil
}
