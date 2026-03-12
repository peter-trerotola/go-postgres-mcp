package server

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/peter-trerotola/go-postgres-mcp/internal/config"
	"github.com/peter-trerotola/go-postgres-mcp/internal/knowledgemap"
	"github.com/peter-trerotola/go-postgres-mcp/internal/postgres"
)

type App struct {
	cfg            *config.Config
	pools          *postgres.PoolManager
	store          *knowledgemap.Store
	mcpServer      *mcpserver.MCPServer
	discoveryOnce  sync.Once
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

func New(cfg *config.Config) (*App, error) {
	store, err := knowledgemap.Open(cfg.KnowledgeMap.Path)
	if err != nil {
		return nil, fmt.Errorf("opening knowledge map: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		cfg:            cfg,
		pools:          postgres.NewPoolManager(),
		store:          store,
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
	}

	hooks := &mcpserver.Hooks{
		OnAfterInitialize: []mcpserver.OnAfterInitializeFunc{
			app.onAfterInitialize,
		},
	}

	mcpSrv := mcpserver.NewMCPServer(
		"go-postgres-mcp",
		"1.0.0",
		mcpserver.WithInstructions(baseInstructions),
		mcpserver.WithResourceCapabilities(false, true),
		mcpserver.WithLogging(),
		mcpserver.WithHooks(hooks),
	)

	app.mcpServer = mcpSrv
	app.registerTools()
	app.registerResources()
	return app, nil
}

// Start connects to all configured databases. Discovery is deferred
// to the OnAfterInitialize hook so the MCP server can respond to the
// initialize request immediately rather than blocking on schema crawl.
func (a *App) Start(ctx context.Context) error {
	for _, dbCfg := range a.cfg.Databases {
		log.Printf("connecting to database %q at %s:%d/%s", dbCfg.Name, dbCfg.Host, dbCfg.Port, dbCfg.Database)
		if err := a.pools.Connect(ctx, dbCfg); err != nil {
			return fmt.Errorf("connecting to %q: %w", dbCfg.Name, err)
		}
		log.Printf("connected to database %q", dbCfg.Name)
	}
	// Always refresh instructions — even without auto-discovery, the knowledge
	// map may contain previously-discovered schema from a prior run.
	a.refreshInstructions()
	return nil
}

// onAfterInitialize is called after the MCP initialize handshake completes.
// It triggers auto-discovery in a background goroutine so that MCP message
// processing is not blocked. Uses sync.Once to ensure discovery runs at most
// once, even if the hook fires multiple times (e.g. reconnects).
func (a *App) onAfterInitialize(_ context.Context, _ any, _ *mcp.InitializeRequest, _ *mcp.InitializeResult) {
	if !a.cfg.KnowledgeMap.AutoDiscoverOnStartup {
		return
	}

	a.discoveryOnce.Do(func() {
		go a.runAutoDiscovery(a.shutdownCtx)
	})
}

// runAutoDiscovery discovers schemas for all configured databases concurrently
// and sends MCP logging notifications to report progress. The context should
// be tied to the server lifecycle so discovery stops on shutdown.
func (a *App) runAutoDiscovery(ctx context.Context) {
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
			a.sendLog(mcp.LoggingLevelInfo, fmt.Sprintf("discovering schema for %s...", dbCfg.Name))
			log.Printf("auto-discovering schema for %q", dbCfg.Name)
			if err := postgres.Discover(ctx, pool, dbCfg, a.store); err != nil {
				log.Printf("warning: auto-discovery failed for %q: %v", dbCfg.Name, err)
				a.sendLog(mcp.LoggingLevelWarning, fmt.Sprintf("schema discovery failed for %s: %v", dbCfg.Name, err))
			} else {
				log.Printf("auto-discovery complete for %q", dbCfg.Name)
			}
		}(pool, dbCfg)
	}
	wg.Wait()

	// Refresh instructions with newly discovered schema
	a.refreshInstructions()

	// Report summary using actual discovered counts from the knowledge map
	tableCount, err := a.store.CountTables()
	if err != nil {
		log.Printf("warning: failed to count tables: %v", err)
		a.sendLog(mcp.LoggingLevelWarning, "schema discovery complete but failed to count tables")
		return
	}
	dbs, dbErr := a.store.ListDatabases()
	dbCount := len(a.cfg.Databases)
	if dbErr == nil {
		dbCount = len(dbs)
	}
	summary := fmt.Sprintf("ready — %d tables across %d databases", tableCount, dbCount)
	a.sendLog(mcp.LoggingLevelInfo, summary)
	log.Printf("auto-discovery complete: %s", summary)
}

// sendLog sends a logging notification to all connected MCP clients.
func (a *App) sendLog(level mcp.LoggingLevel, message string) {
	a.mcpServer.SendNotificationToAllClients(
		"notifications/message",
		map[string]any{
			"level":  level,
			"logger": "go-postgres-mcp",
			"data":   message,
		},
	)
}

// ServeStdio starts the MCP server over stdio transport.
func (a *App) ServeStdio() error {
	return mcpserver.ServeStdio(a.mcpServer)
}

// Shutdown cleans up resources and cancels any in-progress discovery.
func (a *App) Shutdown() {
	if a.shutdownCancel != nil {
		a.shutdownCancel()
	}
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
2. Review the schema summary included in these instructions (under "Schema:") to understand available schemas, tables, and columns before writing queries. Prefer using this upfront schema information rather than calling schema tools for every query.
3. Use query to execute read-only SELECT statements.

Query results include a schema_context field with column names and types for all referenced tables. Use this to verify column names in follow-up queries.

If the schema summary appears incomplete, you suspect the schema has changed, or you need more detailed information about a specific table, you MAY call list_tables or describe_table to fetch fresh schema details. If a query fails with a column or table error, check the schema hint in the error message and the schema summary (or these tools, if needed) for the correct names.`

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

// maxSchemaSummaryBytes is the maximum size of the schema summary appended to
// server instructions. Large databases can produce summaries that exceed LLM
// context limits or slow down transport; this cap ensures a predictable ceiling.
const maxSchemaSummaryBytes = 50_000

// buildSchemaSummary generates a compact text summary of all discovered schemas,
// tables, and columns from the knowledge map. Format:
//
//	Schema:
//	[dbname] schema.table: col1 (type), col2 (type), ...
//
// The summary is truncated at maxSchemaSummaryBytes with a notice appended.
func (a *App) buildSchemaSummary() string {
	dbs, err := a.store.ListDatabases()
	if err != nil || len(dbs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Schema:")
	truncated := false

	for _, db := range dbs {
		if truncated {
			break
		}
		schemas, err := a.store.ListSchemas(db.Name)
		if err != nil {
			continue
		}
		for _, schema := range schemas {
			if truncated {
				break
			}
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
				line := "\n[" + db.Name + "] " + schema.SchemaName + "." + table.TableName + ": " + strings.Join(parts, ", ")
				if b.Len()+len(line) > maxSchemaSummaryBytes {
					truncated = true
					break
				}
				b.WriteString(line)
			}
		}
	}

	if b.Len() <= len("Schema:") {
		return ""
	}
	if truncated {
		b.WriteString("\n... (truncated — use describe_table for full details)")
	}
	return b.String()
}
