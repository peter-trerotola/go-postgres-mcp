# go-postgres-mcp

Read-only PostgreSQL MCP server with schema intelligence.

## Build & Test

```bash
# Run tests
CGO_ENABLED=1 go test ./... -race

# Build via Docker
docker build -t go-postgres-mcp .

# Start local dev environment
docker compose up -d
```

## Project Structure

- `cmd/main.go` — Entry point
- `internal/config/` — YAML config loading and validation
- `internal/guard/` — SQL read-only enforcement (AST parser via pg_query_go)
- `internal/postgres/` — Connection pool, query execution, schema discovery
- `internal/knowledgemap/` — SQLite schema cache with FTS5
- `internal/server/` — MCP server wiring, tool handlers, resource handlers

## Key Conventions

- Branch from `master`, never commit directly
- Use conventional commits (`feat:`, `fix:`, `test:`, etc.)
- Format with `gofmt`, vet with `go vet`
- CGO is required at build time for `pg_query_go`
- All query execution goes through 4-tier read-only protection (see README)
- mcp-go v0.32.0: use `request.GetArguments()` not `request.Params.Arguments`
