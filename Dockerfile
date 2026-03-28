FROM golang:1.25-bookworm AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -o /goro-pg ./cmd/main.go

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

RUN useradd -r -s /bin/false mcpuser

COPY --from=builder /goro-pg /usr/local/bin/goro-pg

USER mcpuser

ENTRYPOINT ["goro-pg"]
CMD ["serve", "--config", "/etc/goro-pg/config.yaml"]
