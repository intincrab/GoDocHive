package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/standard"
	"golang.org/x/net/html"
)

// Document is
type Document struct {
	Title   string
	Content string
	URL     string
}

// List of allowed file extensions
var allowedExtensions = []string{".html", ".htm", ".txt", ".md"}

var index bleve.Index

var root string

func main() {
	var err error
	  
    // current working directory where the binary is run
    currentDir, err := os.Getwd()
    if err != nil {
        log.Fatalf("Error getting current working directory: %v", err)
    }

    path := flag.String("path", currentDir, "Path to the directory")
    refresh := flag.Bool("refresh", false, "refresh/rebuild the index")
    extensions := flag.String("extensions", "", "Comma-separated list of file extensions to include")

    flag.Parse()

    root = *path
	if *extensions != "" {
		allowedExtensions = strings.Split(*extensions, ",")
		for i, ext := range allowedExtensions {
			allowedExtensions[i] = strings.TrimSpace(ext)
			if !strings.HasPrefix(allowedExtensions[i], ".") {
				allowedExtensions[i] = "." + allowedExtensions[i]
			}
		}
	}

	fmt.Println("Using path:", *path)
	fmt.Println("Rebuild the index ? :", *refresh)
	fmt.Println("Allowed extensions:", allowedExtensions)

	indexPath := "index.bleve"
	index, err = bleve.Open(indexPath)
	if *refresh {

		if _, err := os.Stat(indexPath); err == nil {

			err = os.RemoveAll(indexPath)
			if err != nil {
				log.Fatalf("Error deleting existing index: %v", err)
			}
		} else if !os.IsNotExist(err) {
			log.Fatalf("Error checking index path: %v", err)
		}

	}
	if err == bleve.ErrorIndexPathDoesNotExist {
		indexMapping := bleve.NewIndexMapping()
		documentMapping := bleve.NewDocumentMapping()

		textFieldMapping := bleve.NewTextFieldMapping()
		textFieldMapping.Analyzer = standard.Name

		documentMapping.AddFieldMappingsAt("Title", textFieldMapping)
		documentMapping.AddFieldMappingsAt("Content", textFieldMapping)
		documentMapping.AddFieldMappingsAt("URL", textFieldMapping)

		indexMapping.AddDocumentMapping("document", documentMapping)

		index, err = bleve.New("index.bleve", indexMapping)
		if err != nil {
			log.Fatal(err)
		}
		buildIndex(root)
	} else if err != nil {
		log.Fatal(err)
	}
	defer index.Close()

	http.HandleFunc("/", serveFiles)
	http.HandleFunc("/search", handleSearch)

	fmt.Println("Server running at http://localhost:3030")
	log.Fatal(http.ListenAndServe(":3030", nil))
}

// Helper function to check if a file has an allowed extension
// func hasAllowedExtension(filename string) bool {
// 	for _, ext := range allowedExtensions {
// 		if strings.HasSuffix(filename, ext) {
// 			return true
// 		}
// 	}
// 	return false
// }

func hasAllowedExtension(filename string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
}

func buildIndex(root string) {
	batch := index.NewBatch()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && hasAllowedExtension(info.Name(), allowedExtensions) {
			// if !info.IsDir() && strings.HasSuffix(info.Name(), ".html") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			title, bodyContent := extractTitleAndContent(string(content))
			if title == "" {
				title = info.Name()
			}

			doc := Document{
				Title:   title,
				Content: bodyContent,
				URL:     path,
			}

			err = batch.Index(path, doc)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	err = index.Batch(batch)
	if err != nil {
		log.Fatal(err)
	}
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

func serveFiles(w http.ResponseWriter, r *http.Request) {
    filePath := filepath.Join(root, r.URL.Path)
    http.ServeFile(w, r, filePath)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	var results []Document

	if query != "" {
		searchQuery := bleve.NewMatchQuery(query)
		searchRequest := bleve.NewSearchRequest(searchQuery)
		searchRequest.Fields = []string{"Title", "Content", "URL"}
		searchRequest.Highlight = bleve.NewHighlight()
		searchResult, err := index.Search(searchRequest)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, hit := range searchResult.Hits {
			relativeURL, err := filepath.Rel(root, hit.Fields["URL"].(string))
			if err != nil {
				log.Printf("Error creating relative URL: %v", err)
				continue
			}
			doc := Document{
				Title:   hit.Fields["Title"].(string),
				Content: hit.Fields["Content"].(string),
				URL:     relativeURL,
			}
			results = append(results, doc)
		}
	}

	tmpl := template.New("search")

	tmpl.Funcs(template.FuncMap{
		"truncate": func(s string, l int) string {
			if len(s) > l {
				return s[:l] + "..."
			}
			return s
		},
	})

	tmpl, err := tmpl.Parse(`
<!DOCTYPE html>
<html>
<head>
    <title>Go Doc Server :: Search</title>
</head>
<body>
    <div class="row">
        <form action="/search" method="GET">
            <input type="search" id="search_textbox" name="q" value="{{.Query}}">
            <button type="submit">Search</button>
        </form>
    </div>
    <ul>
        {{range .Results}}
        <li>
            <h3><a href="/{{.URL}}">{{.Title}}</a></h3>
            <p>{{truncate .Content 150}}</p>

        </li>
        {{end}}
    </ul>
    <style>
        .row {
            padding: 1%;
        }
    </style>
</body>
</html>
`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Query   string
		Results []Document
	}{
		Query:   query,
		Results: results,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
