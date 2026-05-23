// Package search runs queries against a Bleve index and maps hits back to
// documents with URLs relative to the served root.
package search

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
)

// DefaultPageSize is the number of hits returned per page when the caller
// does not specify a size.
const DefaultPageSize = 10

// Searcher executes queries against an index.
type Searcher struct {
	Index bleve.Index
	Root  string
}

// Hit is a single search result. Snippet is an HTML fragment with matched
// terms wrapped in <mark>; it is empty when the match was not in the body.
type Hit struct {
	Title   string
	URL     string
	Content string
	Snippet string
}

// Result is a single page of search hits plus the paging metadata needed to
// render navigation.
type Result struct {
	Documents []Hit
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
	// The "html" highlighter wraps matched terms in <mark> and HTML-escapes
	// the surrounding text, so the fragment is safe to render as HTML.
	req.Highlight = bleve.NewHighlightWithStyle("html")
	req.Size = size
	req.From = (page - 1) * size

	res, err := s.Index.Search(req)
	if err != nil {
		return Result{}, err
	}

	documents := make([]Hit, 0, len(res.Hits))
	for _, hit := range res.Hits {
		relativeURL, err := filepath.Rel(s.Root, hit.Fields["URL"].(string))
		if err != nil {
			slog.Warn("skipping hit: cannot make relative URL", "url", hit.Fields["URL"], "err", err)
			continue
		}
		var snippet string
		if frags := hit.Fragments["Content"]; len(frags) > 0 {
			snippet = strings.Join(frags, " … ")
		}
		documents = append(documents, Hit{
			Title:   hit.Fields["Title"].(string),
			Content: hit.Fields["Content"].(string),
			URL:     relativeURL,
			Snippet: snippet,
		})
	}

	return Result{Documents: documents, Total: res.Total, Page: page, PageSize: size}, nil
}
