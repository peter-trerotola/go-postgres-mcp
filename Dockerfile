FROM golang:1.23-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -o /go-postgres-mcp ./cmd/main.go

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

RUN useradd -r -s /bin/false mcpuser

COPY --from=builder /go-postgres-mcp /usr/local/bin/go-postgres-mcp

USER mcpuser

ENTRYPOINT ["go-postgres-mcp"]
CMD ["-config", "/etc/go-postgres-mcp/config.yaml"]
