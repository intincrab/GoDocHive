# contributing

thanks for your interest in improving GoDocHive! this is a small project, so the process is light.

## getting started

1. fork and clone the repo.
2. make sure you have Go 1.25+ installed.
3. build and run the tests:
   ```
   go build -o hiver ./cmd/hiver
   go test ./...
   ```
4. run it against a folder of docs to try your change:
   ```
   ./hiver -path /path/to/docs
   ```
   then open http://127.0.0.1:3030/search

## project layout

- `cmd/hiver` — the entrypoint (flags, config, server lifecycle)
- `internal/index` — index lifecycle, document extraction, the live-reindex watcher
- `internal/search` — query execution and result mapping
- `internal/server` — http handlers, middleware, embedded templates/assets, metrics

## before opening a pull request

- run the full audit (the same checks CI runs):
  ```
  make audit
  ```
  this runs gofmt/vet, staticcheck, govulncheck, and the race-enabled tests.
- keep changes focused — one logical change per pull request.
- add or update tests for any behavior you change.
- follow the existing style: small packages under `internal/`, structured logging via `log/slog`, and no new package-level mutable state (pass dependencies explicitly).

## commit messages

use short, present-tense messages with a type prefix, e.g. `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `ci:`. this keeps the history readable and feeds the release changelog.

## reporting bugs and requesting features

open an issue using one of the templates. for bugs, include the exact command and flags you ran, your OS, what you expected, and what actually happened.

## code of conduct

be respectful and constructive. that's the whole policy.
