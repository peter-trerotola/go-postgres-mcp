package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/peter-trerotola/go-postgres-mcp/internal/config"
	"github.com/peter-trerotola/go-postgres-mcp/internal/knowledgemap"
	"github.com/peter-trerotola/go-postgres-mcp/internal/postgres"
)

type App struct {
	cfg       *config.Config
	pools     *postgres.PoolManager
	store     *knowledgemap.Store
	mcpServer *mcpserver.MCPServer
}

func New(cfg *config.Config) (*App, error) {
	store, err := knowledgemap.Open(cfg.KnowledgeMap.Path)
	if err != nil {
		return nil, fmt.Errorf("opening knowledge map: %w", err)
	}

	mcpSrv := mcpserver.NewMCPServer(
		"go-postgres-mcp",
		"1.0.0",
		mcpserver.WithInstructions(baseInstructions),
		mcpserver.WithResourceCapabilities(false, true),
	)

	app := &App{
		cfg:       cfg,
		pools:     postgres.NewPoolManager(),
		store:     store,
		mcpServer: mcpSrv,
	}

	app.registerTools()
	app.registerResources()
	return app, nil
}

// Start connects to all databases and optionally runs auto-discovery.
// Connections are established sequentially (required before discovery),
// then discovery runs concurrently across all databases.
func (a *App) Start(ctx context.Context) error {
	for _, dbCfg := range a.cfg.Databases {
		log.Printf("connecting to database %q at %s:%d/%s", dbCfg.Name, dbCfg.Host, dbCfg.Port, dbCfg.Database)
		if err := a.pools.Connect(ctx, dbCfg); err != nil {
			return fmt.Errorf("connecting to %q: %w", dbCfg.Name, err)
		}
		log.Printf("connected to database %q", dbCfg.Name)
	}

	if a.cfg.KnowledgeMap.AutoDiscoverOnStartup {
		var wg sync.WaitGroup
		for _, dbCfg := range a.cfg.Databases {
			pool, err := a.pools.Get(dbCfg.Name)
			if err != nil {
				log.Printf("warning: auto-discovery skipped for %q: failed to get pool: %v", dbCfg.Name, err)
				continue
			}
			wg.Add(1)
			go func(pool *pgxpool.Pool, dbCfg config.DatabaseConfig) {
				defer wg.Done()
				log.Printf("auto-discovering schema for %q", dbCfg.Name)
				if err := postgres.Discover(ctx, pool, dbCfg, a.store); err != nil {
					log.Printf("warning: auto-discovery failed for %q: %v", dbCfg.Name, err)
				} else {
					log.Printf("auto-discovery complete for %q", dbCfg.Name)
				}
			}(pool, dbCfg)
		}
		wg.Wait()
		a.refreshInstructions()
	}
	return nil
}

// ServeStdio starts the MCP server over stdio transport.
func (a *App) ServeStdio() error {
	return mcpserver.ServeStdio(a.mcpServer)
}

// Shutdown cleans up resources.
func (a *App) Shutdown() {
	a.pools.Close()
	if a.store != nil {
		a.store.Close()
	}
}

// MCPServer returns the underlying MCP server for testing.
func (a *App) MCPServer() *mcpserver.MCPServer {
	return a.mcpServer
}

// baseInstructions is the static portion of the MCP server instructions.
const baseInstructions = `This server provides read-only access to PostgreSQL databases.

Workflow:
1. Use list_databases to see available databases.
2. Use list_tables or describe_table to understand schema BEFORE writing queries.
3. Use query to execute read-only SELECT statements.

Query results include a schema_context field with column names and types for all referenced tables. Use this to verify column names in follow-up queries.

If a query fails with a column or table error, check the schema hint in the error message for correct names.`

// refreshInstructions rebuilds the MCP server instructions with a compact
// schema summary from the knowledge map. This gives the LLM upfront knowledge
// of every table and column without requiring extra tool calls.
func (a *App) refreshInstructions() {
	summary := a.buildSchemaSummary()
	instructions := baseInstructions
	if summary != "" {
		instructions += "\n\n" + summary
	}
	// Apply the WithInstructions option to update the unexported field.
	mcpserver.WithInstructions(instructions)(a.mcpServer)
	log.Printf("server instructions refreshed (%d bytes)", len(instructions))
}

// buildSchemaSummary generates a compact text summary of all discovered schemas,
// tables, and columns from the knowledge map. Format:
//
//	Schema:
//	[dbname] schema.table: col1 (type), col2 (type), ...
func (a *App) buildSchemaSummary() string {
	dbs, err := a.store.ListDatabases()
	if err != nil || len(dbs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Schema:")

	for _, db := range dbs {
		schemas, err := a.store.ListSchemas(db.Name)
		if err != nil {
			continue
		}
		for _, schema := range schemas {
			tables, err := a.store.ListTables(db.Name, schema.SchemaName)
			if err != nil {
				continue
			}
			for _, table := range tables {
				cols, err := a.store.ListColumnsCompact(db.Name, schema.SchemaName, table.TableName)
				if err != nil {
					continue
				}
				parts := make([]string, len(cols))
				for i, c := range cols {
					parts[i] = c.Column + " (" + c.Type + ")"
				}
				b.WriteString("\n[")
				b.WriteString(db.Name)
				b.WriteString("] ")
				b.WriteString(schema.SchemaName)
				b.WriteString(".")
				b.WriteString(table.TableName)
				b.WriteString(": ")
				b.WriteString(strings.Join(parts, ", "))
			}
		}
	}

	if b.Len() <= len("Schema:") {
		return ""
	}
	return b.String()
}
