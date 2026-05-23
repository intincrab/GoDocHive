# GoDocHive

a simple document server/search engine for HTML docs.

powered by Go + [Bleve](https://github.com/blevesearch/bleve) + html/template

## setting it up on your system

### prerequisites

- [Go](https://go.dev/dl/) 1.23 or newer — only needed if you build from source
- git — only needed to clone the repository

### option a: build from source

1. clone the repository:
   ```
   git clone https://github.com/intincrab/GoDocHive.git
   cd GoDocHive
   ```

2. build the binary:
   ```
   make build
   ```
   or, without make:
   ```
   go build -o hiver ./cmd/hiver
   ```
   this produces a single self-contained binary named `hiver` (`hiver.exe` on windows). there are no runtime dependencies to install.

3. (optional) put it on your PATH so you can run `hiver` from any folder:
   - **linux / macOS:** move it into a directory already on your PATH, e.g. `sudo mv hiver /usr/local/bin/`
   - **windows:** move `hiver.exe` into a folder of your choice, add that folder to your PATH (settings → environment variables), then open a fresh terminal

### option b: download a prebuilt binary

download the binary for your platform from [releases](https://github.com/intincrab/GoDocHive/releases) and place it wherever is convenient — ideally on your PATH, as above.

## usage

1. point it at a folder of docs. the simplest way is to `cd` into that folder and run:
   ```
   hiver
   ```
   by default it indexes the current folder and serves on `http://127.0.0.1:3030`.

2. or run it from anywhere and pass the folder explicitly:
   ```
   hiver -path /path/to/your/docs
   ```

3. open a web browser at `http://127.0.0.1:3030/search` to use the search interface.

the first run builds a search index (stored in an `index.bleve` folder next to where you run it). while running, it watches the folder and re-indexes changes automatically, so edits show up in search within about a second. pass `-refresh` to rebuild the index from scratch. press Ctrl+C to stop the server cleanly.

## available flags

| flag | description | default value |
|------|-------------|---------------|
| `-path` | directory to index and serve | current working directory |
| `-refresh` | rebuild the search index on start | `false` |
| `-extensions` | comma-separated list of file extensions to include | `.html,.htm,.txt,.md` |
| `-addr` | address to listen on (`host:port`) | `127.0.0.1:3030` |
| `-index` | path to the bleve index directory | `index.bleve` |
| `-watch` | watch the path and re-index changes live | `true` |

flags can also be set via environment variables (the flag wins if both are given): `ADDR` (or `PORT`, used as `127.0.0.1:$PORT`) and `INDEX_PATH`.

## security & deployment

GoDocHive has **no authentication** and serves arbitrary on-disk files from `-path`. by default it binds to `127.0.0.1`, so it is only reachable from the local machine.

to expose it on a network, do **not** simply bind to `0.0.0.0` and walk away. put it behind a reverse proxy (nginx, Caddy, Traefik) that terminates TLS and handles authentication, then point the proxy at the loopback listener. only serve document trees you trust — there is no sandbox, and symlinks inside the served directory are followed.

## development

run the tests:
```
go test ./...
```

run the full audit (formatting, vet, staticcheck, vulnerability scan, race tests):
```
make audit
```
