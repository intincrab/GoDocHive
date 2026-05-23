# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.23 AS build
WORKDIR /src

# Download modules first so this layer is cached unless go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/hiver ./cmd/hiver

# ---- final stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /home/nonroot
COPY --from=build /out/hiver /usr/local/bin/hiver

EXPOSE 3030

# Inside a container the trust boundary is the container/network, so bind to
# all interfaces; the operator still decides whether to publish the port.
# Mount your docs at /docs (read-only is fine). The index is written under
# /home/nonroot (owned by the nonroot user); mount a volume there to persist
# it across restarts.
#
#   docker build -t godochive .
#   docker run --rm -p 3030:3030 -v "$PWD/docs:/docs:ro" godochive
#
ENTRYPOINT ["/usr/local/bin/hiver"]
CMD ["-addr", "0.0.0.0:3030", "-path", "/docs", "-index", "/home/nonroot/index.bleve"]
