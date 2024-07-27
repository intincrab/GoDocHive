# GoDocHive

a simple document server/search engine for HTML docs.  

powered by Go + [Bleve](https://github.com/blevesearch/bleve) + html/template

## usage

1. download the binary from [releases](https://github.com/intincrab/docuverse/releases) and add it to the root of your documentation or site folder

2. run the server:
   ```
   ./hiver
   ```

3. open a web browser and navigate to `http://localhost:3030/search` to use the search interface.

## available flags

| flag | description | default value |
|------|-------------|---------------|
| `-path` | Specifies the directory to index and serve | Current working directory |
| `-refresh` | Rebuilds the search index | `false` |
| `-extensions` | Sets allowed file extensions | ".html,.htm,.txt,.md" |

## installation

1. clone the repository:
   ```
   git clone https://github.com/intincrab/GoDocHive.git
   cd GoDocHive
   ```

2. build the project:
   ```
   make build
   go build
   ```
   
