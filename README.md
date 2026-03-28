# goro-pg

[![CI](https://github.com/peter-trerotola/goro-pg/actions/workflows/ci.yml/badge.svg)](https://github.com/peter-trerotola/goro-pg/actions/workflows/ci.yml)
[![Release](https://github.com/peter-trerotola/goro-pg/actions/workflows/release.yml/badge.svg)](https://github.com/peter-trerotola/goro-pg/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/peter-trerotola/goro-pg)](https://goreportcard.com/report/github.com/peter-trerotola/goro-pg)
[![Go Reference](https://pkg.go.dev/badge/github.com/peter-trerotola/goro-pg.svg)](https://pkg.go.dev/github.com/peter-trerotola/goro-pg)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

```
                           .-------------------------.
                           | how many signups today? |
                           '--.----------------------'
                              |
   ____  ______  ___          ,_---~~~~~----._
  /    )/      \/   \      _,,_,*^____    ___``*g*\"*,
 (     / __    _\    )    / __/ /'    ^. /     \ ^@q  f
  \    (/ o)  ( o)   )   [  @f | @))   || @))   l 0 _/
   \_  (_  )   \ )  /     \`/   \~___ / _ \____/   \
     \  /\_/    \)_/        |          _l_l_         I
      \/  //|  |\           }         [_____]        I
          v |  | v          ]           | | |        |
            \__/            ]            ~ ~         |
                            |                        |
                             |                       |
```

**Go + Read-Only + Postgres.** A CLI tool and MCP server for exploring PostgreSQL databases with schema intelligence. All access is read-only with 4 layers of protection.

## Quick Start

```bash
# Configure your databases
cp config.example.yaml config.yaml
# Edit config.yaml with your database details

# Discover schemas
export PROD_DB_PASSWORD="your_password"
goro-pg discover

# Explore
goro-pg databases
goro-pg tables -d mydb
goro-pg describe -d mydb users
goro-pg search "user email"

# Query
goro-pg query -d mydb "SELECT * FROM users LIMIT 10"

# Pipe-friendly
echo "SELECT count(*) FROM orders" | goro-pg query -d mydb -
goro-pg tables -d mydb --format json | jq .
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `goro-pg query <sql>` | Execute a read-only SQL query |
| `goro-pg discover [database]` | Discover/refresh database schemas |
| `goro-pg databases` | List configured databases |
| `goro-pg schemas [database]` | List schemas in a database |
| `goro-pg tables [database]` | List tables in a schema |
| `goro-pg describe [database] <table>` | Full table detail (columns, constraints, indexes, FKs) |
| `goro-pg views [database]` | List views in a schema |
| `goro-pg functions [database]` | List functions in a schema |
| `goro-pg search <query>` | Full-text search across all schema metadata |
| `goro-pg serve` | Start MCP stdio server |
| `goro-pg version` | Print version |

### Global Flags

```
-c, --config     Config file path (default: config.yaml, env: GORO_PG_CONFIG)
-d, --database   Default database name (env: GORO_PG_DATABASE)
-f, --format     Output format: table, json, csv, plain (auto-detects TTY)
```

### Output Formats

- **table** (default for TTY) — psql-style aligned columns
- **json** — machine-readable JSON
- **csv** — comma-separated values
- **plain** (default for pipes) — tab-separated, no headers

## Features

- **CLI-first** — use directly from the terminal, no MCP client required
- **MCP server** — also works as an MCP server via `goro-pg serve`
- **4 layers of read-only protection** to prevent any data mutation
- **Schema knowledge map** stored in SQLite with full-text search (FTS5)
- **Automatic schema context** injected into query responses
- **Enriched error messages** with actual schema when queries fail
- **Multi-database support** from a single config
- **Auto-discovery** of schemas, tables, columns, constraints, indexes, views, and functions
- **Schema and table filtering** — whitelist or blacklist what gets discovered

## Read-Only Protection

Every query passes through four defensive layers before execution:

| Layer | Mechanism | Description |
|-------|-----------|-------------|
| **Tier 1** | AST parser | Parses SQL using PostgreSQL's actual parser (`pg_query_go`) and validates only SELECT statements are present. Rejects SELECT INTO, FOR UPDATE/SHARE, CTEs with mutations |
| **Tier 2** | Connection-level | Every pgx pool connection sets `default_transaction_read_only=on` via RuntimeParams |
| **Tier 3** | Transaction-level | Every query runs inside `BEGIN READ ONLY` via `pgx.TxOptions{AccessMode: pgx.ReadOnly}` |
| **Tier 4** | PostgreSQL user | Configure with a database user that has only SELECT grants (see configuration below) |

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

> **Note:** These filters are enforced at both discovery time (what enters the knowledge map) and query time (the `query` command extracts table references from SQL via AST parsing and rejects queries that reference filtered-out schemas or tables). For defense-in-depth, also configure PostgreSQL grants (Tier 4) to restrict access at the database level.

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

Download from [GitHub Releases](https://github.com/peter-trerotola/goro-pg/releases):

```bash
# Linux amd64
curl -L https://github.com/peter-trerotola/goro-pg/releases/latest/download/goro-pg_linux_amd64.tar.gz | tar xz

# Linux arm64
curl -L https://github.com/peter-trerotola/goro-pg/releases/latest/download/goro-pg_linux_arm64.tar.gz | tar xz

# macOS Apple Silicon
curl -L https://github.com/peter-trerotola/goro-pg/releases/latest/download/goro-pg_darwin_arm64.tar.gz | tar xz

# macOS Intel
curl -L https://github.com/peter-trerotola/goro-pg/releases/latest/download/goro-pg_darwin_amd64.tar.gz | tar xz
```

### Docker

```bash
docker pull ghcr.io/peter-trerotola/goro-pg:latest
```

### Build from source

Requires Go 1.23+ and a C compiler (for `pg_query_go`):

```bash
CGO_ENABLED=1 go build -o goro-pg ./cmd/main.go
```

## Docker Compose (development)

```bash
docker compose up
```

This starts a PostgreSQL instance and goro-pg in MCP server mode with auto-discovery enabled.

## MCP Server Mode

goro-pg also works as an MCP (Model Context Protocol) server for use with Claude Desktop, Claude Code, and other MCP-compatible clients:

```bash
goro-pg serve --config config.yaml
```

### MCP Tools

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

### MCP Resources

| Template | Description |
|----------|-------------|
| `schema:///{database}/tables` | List all tables with column counts |
| `schema:///{database}/{schema}/{table}` | Full table detail (columns, constraints, indexes, FKs) |

### Claude Desktop / Claude Code Integration

Add to your MCP settings:

```json
{
  "mcpServers": {
    "postgres": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "PROD_DB_PASSWORD",
        "-v", "/path/to/config.yaml:/etc/goro-pg/config.yaml:ro",
        "goro-pg"
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
      "command": "/path/to/goro-pg",
      "args": ["serve", "--config", "/path/to/config.yaml"],
      "env": {
        "PROD_DB_PASSWORD": "your_password"
      }
    }
  }
}
```

### Schema Context Injection

LLMs often write queries with wrong column names. goro-pg addresses this at multiple layers:

1. **Server instructions** — workflow guidance sent during MCP initialization
2. **Schema context in responses** — every `query` response includes column names/types for referenced tables
3. **Enriched errors** — failed queries include actual schema from the knowledge map

## Testing

```bash
# Run all unit tests
CGO_ENABLED=1 go test ./... -race

# Run only guard (read-only enforcement) tests
go test ./internal/guard/... -v

# Run only knowledge map tests
go test ./internal/knowledgemap/... -v
```

## Contributing Adversarial Tests

The file `internal/guard/adversarial_test.go` contains ~200 test cases that attempt to bypass the read-only guard. Each case is a simple struct:

```go
type adversarialCase struct {
    name string // descriptive name for the test
    sql  string // the SQL to test
    tier string // which tier blocks it: "tier1", "tier2", "tier3", "tier4"
}
```

```bash
go test ./internal/guard/ -run TestAdversarial -v
```

If you find SQL that bypasses all four tiers, please open an issue.

## Project Structure

```
goro-pg/
├── cmd/
│   └── main.go                      # Entry point (Cobra bootstrap)
├── internal/
│   ├── cli/                         # CLI commands + output formatting
│   │   ├── root.go                  # Root command, global flags
│   │   ├── query.go                 # query subcommand
│   │   ├── discover.go              # discover subcommand
│   │   ├── databases.go             # databases subcommand
│   │   ├── schemas.go               # schemas subcommand
│   │   ├── tables.go                # tables subcommand
│   │   ├── describe.go              # describe subcommand
│   │   ├── views.go                 # views subcommand
│   │   ├── functions.go             # functions subcommand
│   │   ├── search.go                # search subcommand
│   │   ├── serve.go                 # serve subcommand (MCP mode)
│   │   ├── version.go               # version subcommand
│   │   └── format.go                # table/json/csv/plain formatters
│   ├── engine/                      # Shared business logic
│   │   └── engine.go                # Query, discover, schema lookup orchestration
│   ├── config/
│   │   └── config.go                # YAML config types + loading + validation
│   ├── guard/
│   │   ├── parser.go                # Tier 1: AST validation + table ref extraction
│   │   ├── guard.go                 # Guard entry point + ForbiddenError type
│   │   └── adversarial_test.go      # ~200 adversarial bypass attempt tests
│   ├── postgres/
│   │   ├── pool.go                  # Connection pool manager (Tier 2)
│   │   ├── readonly.go              # Guarded query execution (Tier 3)
│   │   └── discovery.go             # Schema discovery with filtering
│   ├── knowledgemap/
│   │   ├── store.go                 # SQLite CRUD operations
│   │   ├── query.go                 # Knowledge map query methods
│   │   └── schema.sql               # SQLite DDL (tables, FTS5)
│   └── server/
│       ├── server.go                # MCP server wiring
│       ├── tools.go                 # MCP tool definitions + handlers
│       └── resources.go             # MCP resource template handlers
├── config.example.yaml
├── Dockerfile
├── docker-compose.yaml
└── .goreleaser.yml
```

## Architecture

goro-pg has two interfaces (CLI and MCP server) built on a shared engine layer:

```
CLI (cobra) ──→ Engine ←── MCP Server (mcp-go)
                  │
           ┌──────┼──────┐
           ↓      ↓      ↓
        Config  Guard  Postgres  KnowledgeMap
                  │        │          │
                  ↓        ↓          ↓
               pg_query   pgx      SQLite
```

Schema metadata is crawled from PostgreSQL and cached in a local SQLite database (the "knowledge map"), which enables instant schema lookups and full-text search without hitting the live database.

The SQL guard uses `pg_query_go` which wraps PostgreSQL's actual parser (`libpg_query`). This means SQL validation uses the same parser as PostgreSQL itself — no ambiguity about what constitutes a SELECT vs. a mutation.
