# go-postgres-mcp

A Go-based [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server that provides **read-only** access to PostgreSQL databases. Designed for use with Claude Desktop, Claude Code, and other MCP-compatible clients.

## Features

- **9 MCP tools** for querying and exploring PostgreSQL schemas
- **4 layers of read-only protection** to prevent any data mutation
- **Schema knowledge map** stored in SQLite with full-text search (FTS5)
- **Automatic schema context** injected into query responses so LLMs always see correct column names
- **Enriched error messages** that include actual schema when queries fail with wrong column/table names
- **MCP resource templates** for browsable schema access
- **Multi-database support** from a single server instance
- **Auto-discovery** of schemas, tables, columns, constraints, indexes, views, and functions
- **Schema and table filtering** — whitelist or blacklist what gets discovered
- **Docker-ready** with multi-stage build

## Read-Only Protection

Every query passes through four defensive layers before execution:

| Layer | Mechanism | Description |
|-------|-----------|-------------|
| **Tier 1** | AST parser | Parses SQL using PostgreSQL's actual parser (`pg_query_go`) and validates only SELECT statements are present. Rejects SELECT INTO, FOR UPDATE/SHARE, CTEs with mutations |
| **Tier 2** | Connection-level | Every pgx pool connection sets `default_transaction_read_only=on` via RuntimeParams |
| **Tier 3** | Transaction-level | Every query runs inside `BEGIN READ ONLY` via `pgx.TxOptions{AccessMode: pgx.ReadOnly}` |
| **Tier 4** | PostgreSQL user | Configure with a database user that has only SELECT grants (see configuration below) |

## Schema Context Injection

LLMs often write queries with wrong column names — either because they skip `describe_table` or lose schema details across conversation turns. This server addresses that with automatic schema context at multiple layers:

### 1. Server Instructions

The server sends workflow guidance to the LLM during initialization, directing it to use `describe_table` before writing queries and to check `schema_context` in responses.

### 2. Schema Context in Query Responses

Every successful `query` response includes a `schema_context` field with column names and types for all tables referenced in the SQL:

```json
{
  "columns": ["id", "name", "email"],
  "rows": [...],
  "count": 10,
  "truncated": false,
  "schema_context": {
    "public.users": [
      {"column": "id", "type": "integer"},
      {"column": "name", "type": "text"},
      {"column": "email", "type": "text"}
    ]
  }
}
```

Table references are extracted from the SQL via AST parsing (the same `pg_query_go` parser used for read-only enforcement) and looked up from the in-process SQLite knowledge map — no additional database round-trips.

### 3. Enriched Error Messages

When a query fails with a column or table "does not exist" error, the error message is enriched with the actual schema from the knowledge map:

```
executing query: ERROR: column "source_page_id" does not exist (SQLSTATE 42703)

Schema for referenced tables:
  public.spider_links: id (bigint), job_id (uuid), from_page_id (bigint), to_url (text)
```

This gives the LLM immediate feedback to self-correct without needing a separate `describe_table` call.

### 4. Tool Annotations

All tools are annotated with MCP tool hints (`readOnlyHint`, `destructiveHint`, `idempotentHint`) so clients can make informed decisions about tool usage. All tools except `discover` are marked read-only.

## MCP Tools

| Tool | Description | Data Source |
|------|-------------|-------------|
| `query` | Execute a read-only SELECT query | PostgreSQL (live) |
| `discover` | Discover/refresh schema for a database | PostgreSQL -> SQLite |
| `list_databases` | List all configured databases | SQLite knowledge map |
| `list_schemas` | List schemas in a database | SQLite knowledge map |
| `list_tables` | List tables in a schema | SQLite knowledge map |
| `describe_table` | Full column/constraint/index/FK detail | SQLite knowledge map |
| `list_views` | List views in a schema | SQLite knowledge map |
| `list_functions` | List functions in a schema | SQLite knowledge map |
| `search_schema` | Full-text search across all metadata | SQLite FTS5 |

## MCP Resources

The server exposes schema as browsable [MCP resources](https://modelcontextprotocol.io/docs/concepts/resources) via URI templates:

| Template | Description |
|----------|-------------|
| `schema:///{database}/tables` | List all tables with column counts |
| `schema:///{database}/{schema}/{table}` | Full table detail (columns, constraints, indexes, FKs) |

These are available to MCP clients that support resource browsing.

## Configuration

Create a `config.yaml` (see `config.example.yaml`):

```yaml
databases:
  - name: "production"
    host: "db.example.com"
    port: 5432
    database: "myapp"
    user: "readonly_user"
    password_env: "PROD_DB_PASSWORD"    # resolved from environment variable
    sslmode: "require"

knowledgemap:
  path: "./knowledgemap.db"
  auto_discover_on_startup: true
```

**Important:** The `password_env` field references an environment variable name, never a raw password. The server will refuse to start if the variable is unset or empty.

### Discovery Filtering

You can optionally control what gets discovered and indexed into the knowledge map.

**Schema filter** — only discover specific schemas (all non-system schemas if omitted):

```yaml
databases:
  - name: "production"
    host: "db.example.com"
    database: "myapp"
    user: "readonly_user"
    password_env: "PROD_DB_PASSWORD"
    schemas:
      - "public"
      - "billing"
```

**Table whitelist** — only discover specific tables:

```yaml
    tables:
      include:
        - "public.users"
        - "public.orders"
        - "billing.invoices"
```

**Table blacklist** — discover everything except specific tables:

```yaml
    tables:
      exclude:
        - "public.migrations"
        - "public.sessions"
```

`include` and `exclude` are mutually exclusive. Table names must be in `schema.table` format.

Schema and table filters can be combined — schema filtering is applied first, then table filtering within those schemas.

> **Note:** These filters control what the knowledge map tools (`list_tables`, `describe_table`, `search_schema`, etc.) can see. The `query` tool executes live SQL and can still access any table the database user has SELECT privileges on. Use PostgreSQL grants (Tier 4) to restrict live query access.

### Creating a read-only PostgreSQL user (Tier 4)

```sql
CREATE ROLE readonly_user WITH LOGIN PASSWORD 'strong_password_here';
GRANT CONNECT ON DATABASE myapp TO readonly_user;
GRANT USAGE ON SCHEMA public TO readonly_user;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO readonly_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO readonly_user;
```

## Installation

### Prebuilt binaries

Download from [GitHub Releases](https://github.com/peter-trerotola/go-postgres-mcp/releases):

```bash
# Linux amd64
curl -L https://github.com/peter-trerotola/go-postgres-mcp/releases/latest/download/go-postgres-mcp_linux_amd64.tar.gz | tar xz
```

### Docker

```bash
docker pull ghcr.io/peter-trerotola/go-postgres-mcp:latest
```

### Build from source

Requires Go 1.23+ and a C compiler (for `pg_query_go`):

```bash
CGO_ENABLED=1 go build -o go-postgres-mcp ./cmd/main.go
```

## Quick Start

### Docker Compose (development)

```bash
docker compose up
```

This starts a PostgreSQL instance and the MCP server with auto-discovery enabled.

### Run directly

```bash
export PROD_DB_PASSWORD="your_password"
./go-postgres-mcp -config config.yaml
```

## Claude Desktop / Claude Code Integration

Add to your MCP settings:

```json
{
  "mcpServers": {
    "postgres": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "PROD_DB_PASSWORD",
        "-v", "/path/to/config.yaml:/etc/go-postgres-mcp/config.yaml:ro",
        "go-postgres-mcp"
      ]
    }
  }
}
```

Or if running the binary directly:

```json
{
  "mcpServers": {
    "postgres": {
      "command": "/path/to/go-postgres-mcp",
      "args": ["-config", "/path/to/config.yaml"],
      "env": {
        "PROD_DB_PASSWORD": "your_password"
      }
    }
  }
}
```

## Testing

```bash
# Run all unit tests
go test ./...

# Run with verbose output
go test ./... -v

# Run only guard (read-only enforcement) tests
go test ./internal/guard/... -v

# Run only knowledge map tests
go test ./internal/knowledgemap/... -v
```

## Contributing Adversarial Tests

The file `internal/guard/adversarial_test.go` contains ~200 test cases that attempt to bypass the read-only guard. These are organized as table-driven tests, making it easy to add new cases.

### Adding a test case

Each case is a simple struct with three fields:

```go
type adversarialCase struct {
    name string // descriptive name for the test
    sql  string // the SQL to test
    tier string // which tier blocks it: "tier1", "tier2", "tier3", "tier4"
}
```

Find the appropriate test function and slice, then add your case:

```go
// In TestAdversarial_MustBlock — SQL that Tier 1 (AST parser) must reject:
{"my new bypass attempt", "SELECT 1; DROP TABLE pwned", "tier1"},

// In TestAdversarial_FunctionCalls — dangerous functions that pass Tier 1
// but are caught by Tier 2+ (connection/transaction read-only):
{"my dangerous function", "SELECT my_scary_func()", "tier2+"},

// In TestAdversarial_EdgeCases — valid SELECTs that must be allowed:
{"allowed_my complex but safe query", "SELECT ...", "tier1"},
```

### Test categories

| Test function | What it tests |
|---|---|
| `TestAdversarial_MustBlock` | SQL that the AST parser (Tier 1) **must reject** — mutations, DDL, session commands, etc. |
| `TestAdversarial_FunctionCalls` | Dangerous function calls in SELECT (e.g. `pg_terminate_backend`, `set_config`). These pass Tier 1 but are caught by Tiers 2-4. |
| `TestAdversarial_CommentAndEncoding` | Tricks using comments, dollar-quoting, null bytes, whitespace, and encoding |
| `TestAdversarial_EdgeCases` | Complex but valid queries that **must be allowed** (window functions, nested subqueries, CTEs, UNION trees), plus mutations hidden inside complex structures that must be blocked |

### Running just the adversarial tests

```bash
go test ./internal/guard/ -run TestAdversarial -v
```

### Tier reference

- **Tier 1** (AST parser) — `guard.Validate()` rejects non-SELECT statements at parse time
- **Tier 2** (connection) — `default_transaction_read_only=on` on every pgx pool connection
- **Tier 3** (transaction) — `BEGIN READ ONLY` wraps every query execution
- **Tier 4** (PostgreSQL user) — database user with only SELECT grants

If you find SQL that bypasses all four tiers, please open an issue.

## Project Structure

```
go-postgres-mcp/
├── cmd/
│   └── main.go                      # Entry point
├── internal/
│   ├── config/
│   │   ├── config.go                # YAML config types + loading + validation
│   │   └── config_test.go           # Config loading/validation tests
│   ├── guard/
│   │   ├── parser.go                # Tier 1: AST validation + table ref extraction
│   │   ├── parser_test.go           # ExtractTableRefs tests (JOINs, CTEs, etc.)
│   │   ├── guard.go                 # Guard entry point + ForbiddenError type
│   │   ├── guard_test.go            # Comprehensive read-only enforcement tests
│   │   └── adversarial_test.go      # ~200 adversarial bypass attempt tests
│   ├── postgres/
│   │   ├── pool.go                  # Connection pool manager (Tier 2)
│   │   ├── pool_test.go             # Pool manager unit tests
│   │   ├── readonly.go              # Guarded query execution (Tier 3)
│   │   └── discovery.go             # Schema discovery with filtering
│   ├── knowledgemap/
│   │   ├── schema.go                # Embeds SQLite DDL (schema.sql)
│   │   ├── schema.sql               # SQLite DDL (tables, FTS5)
│   │   ├── store.go                 # SQLite CRUD operations (sqlx)
│   │   ├── store_test.go            # Knowledge map CRUD + FTS tests
│   │   └── query.go                 # Knowledge map query methods (sqlx)
│   └── server/
│       ├── server.go                # MCP server wiring + App struct
│       ├── tools.go                 # MCP tool definitions + handlers
│       ├── tools_test.go            # Tool, annotation, schema context tests
│       ├── resources.go             # MCP resource template handlers
│       └── resources_test.go        # Resource handler tests
├── config.example.yaml
├── Dockerfile
├── docker-compose.yaml
├── .gitignore
├── go.mod
└── go.sum
```

## Architecture

The server uses `github.com/mark3labs/mcp-go` for the MCP protocol over stdio transport. Schema metadata is crawled from PostgreSQL and cached in a local SQLite database (the "knowledge map"), which enables instant schema lookups and full-text search without hitting the live database.

The SQL guard uses `pg_query_go` which wraps PostgreSQL's actual parser (`libpg_query`). This means SQL validation uses the same parser as PostgreSQL itself — there's no ambiguity about what constitutes a SELECT vs. a mutation. The same parser is also used to extract table references from queries for schema context injection. CGO is only needed at build time; the Docker multi-stage build handles this cleanly.

Discovery can be scoped using schema and table filters in the config. This controls what enters the knowledge map — useful for large databases where you only need visibility into specific schemas or want to hide internal/migration tables from AI tools.
