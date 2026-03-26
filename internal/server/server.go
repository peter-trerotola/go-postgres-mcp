package server

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/peter-trerotola/goro-pg/internal/config"
	"github.com/peter-trerotola/goro-pg/internal/engine"
	"github.com/peter-trerotola/goro-pg/internal/postgres"
)

type App struct {
	engine         *engine.Engine
	mcpServer      *mcpserver.MCPServer
	discoveryOnce  sync.Once
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

func New(cfg *config.Config) (*App, error) {
	eng, err := engine.New(cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	app := &App{
		engine:         eng,
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
	}

	hooks := &mcpserver.Hooks{
		OnAfterInitialize: []mcpserver.OnAfterInitializeFunc{
			app.onAfterInitialize,
		},
	}

	mcpSrv := mcpserver.NewMCPServer(
		"goro-pg",
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
	if err := a.engine.Connect(ctx); err != nil {
		return err
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
	if !a.engine.Cfg.KnowledgeMap.AutoDiscoverOnStartup {
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
	for _, dbCfg := range a.engine.Cfg.Databases {
		pool, err := a.engine.Pools.Get(dbCfg.Name)
		if err != nil {
			log.Printf("warning: auto-discovery skipped for %q: failed to get pool: %v", dbCfg.Name, err)
			continue
		}
		wg.Add(1)
		go func(pool *pgxpool.Pool, dbCfg config.DatabaseConfig) {
			defer wg.Done()
			a.sendLog(mcp.LoggingLevelInfo, fmt.Sprintf("discovering schema for %s...", dbCfg.Name))
			log.Printf("auto-discovering schema for %q", dbCfg.Name)
			if err := postgres.Discover(ctx, pool, dbCfg, a.engine.Store); err != nil {
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
	tableCount, err := a.engine.Store.CountTables()
	if err != nil {
		log.Printf("warning: failed to count tables: %v", err)
		a.sendLog(mcp.LoggingLevelWarning, "schema discovery complete but failed to count tables")
		return
	}
	dbs, dbErr := a.engine.Store.ListDatabases()
	dbCount := len(a.engine.Cfg.Databases)
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
			"logger": "goro-pg",
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
	a.engine.Shutdown()
}

// MCPServer returns the underlying MCP server for testing.
func (a *App) MCPServer() *mcpserver.MCPServer {
	return a.mcpServer
}

// Engine returns the underlying engine for testing or reuse.
func (a *App) Engine() *engine.Engine {
	return a.engine
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
	summary := a.engine.BuildSchemaSummary()
	instructions := baseInstructions
	if summary != "" {
		instructions += "\n\n" + summary
	}
	// Apply the WithInstructions option to update the unexported field.
	mcpserver.WithInstructions(instructions)(a.mcpServer)
	log.Printf("server instructions refreshed (%d bytes)", len(instructions))
}
