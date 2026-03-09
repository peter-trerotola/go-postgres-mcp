package server

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
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

	app := &App{
		cfg:   cfg,
		pools: postgres.NewPoolManager(),
		store: store,
	}

	hooks := &mcpserver.Hooks{
		OnAfterInitialize: []mcpserver.OnAfterInitializeFunc{
			app.onAfterInitialize,
		},
	}

	mcpSrv := mcpserver.NewMCPServer(
		"go-postgres-mcp",
		"1.0.0",
		mcpserver.WithInstructions(`This server provides read-only access to PostgreSQL databases.

Workflow:
1. Use list_databases to see available databases.
2. Use list_tables or describe_table to understand schema BEFORE writing queries.
3. Use query to execute read-only SELECT statements.

Query results include a schema_context field with column names and types for all referenced tables. Use this to verify column names in follow-up queries.

If a query fails with a column or table error, check the schema hint in the error message for correct names.`),
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
	return nil
}

// onAfterInitialize is called after the MCP initialize handshake completes.
// It triggers auto-discovery in a background goroutine so that MCP message
// processing is not blocked. Progress is reported via MCP logging notifications.
func (a *App) onAfterInitialize(ctx context.Context, id any, msg *mcp.InitializeRequest, result *mcp.InitializeResult) {
	if !a.cfg.KnowledgeMap.AutoDiscoverOnStartup {
		return
	}

	go a.runAutoDiscovery()
}

// runAutoDiscovery discovers schemas for all configured databases concurrently
// and sends MCP logging notifications to report progress.
func (a *App) runAutoDiscovery() {
	ctx := context.Background()

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

	// Report summary
	tableCount, err := a.store.CountTables()
	if err != nil {
		log.Printf("warning: failed to count tables: %v", err)
		tableCount = 0
	}
	dbCount := len(a.cfg.Databases)
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
