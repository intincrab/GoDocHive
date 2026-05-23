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
| `-addr` | Address to listen on (`host:port`) | `127.0.0.1:3030` |
| `-index` | Path to the Bleve index directory | `index.bleve` |

Flags can also be set via environment variables (the flag wins if both are
given): `ADDR` (or `PORT`, used as `127.0.0.1:$PORT`) and `INDEX_PATH`.

## security & deployment

GoDocHive has **no authentication** and serves arbitrary on-disk files from
`-path`. By default it binds to `127.0.0.1`, so it is only reachable from the
local machine.

To expose it on a network, do **not** simply bind to `0.0.0.0` and walk away.
Put it behind a reverse proxy (nginx, Caddy, Traefik) that terminates TLS and
handles authentication, then point the proxy at the loopback listener. Only
serve document trees you trust — there is no sandbox, and symlinks inside the
served directory are followed.

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
   
