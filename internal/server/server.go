package server

import (
	"context"
	"fmt"
	"log"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/petros/go-postgres-mcp/internal/config"
	"github.com/petros/go-postgres-mcp/internal/knowledgemap"
	"github.com/petros/go-postgres-mcp/internal/postgres"
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
	)

	app := &App{
		cfg:       cfg,
		pools:     postgres.NewPoolManager(),
		store:     store,
		mcpServer: mcpSrv,
	}

	app.registerTools()
	return app, nil
}

// Start connects to all databases and optionally runs auto-discovery.
func (a *App) Start(ctx context.Context) error {
	for _, dbCfg := range a.cfg.Databases {
		log.Printf("connecting to database %q at %s:%d/%s", dbCfg.Name, dbCfg.Host, dbCfg.Port, dbCfg.Database)
		if err := a.pools.Connect(ctx, dbCfg); err != nil {
			return fmt.Errorf("connecting to %q: %w", dbCfg.Name, err)
		}
		log.Printf("connected to database %q", dbCfg.Name)

		if a.cfg.KnowledgeMap.AutoDiscoverOnStartup {
			log.Printf("auto-discovering schema for %q", dbCfg.Name)
			pool, err := a.pools.Get(dbCfg.Name)
			if err != nil {
				log.Printf("warning: auto-discovery skipped for %q: failed to get pool: %v", dbCfg.Name, err)
				continue
			}
			if err := postgres.Discover(ctx, pool, dbCfg, a.store); err != nil {
				log.Printf("warning: auto-discovery failed for %q: %v", dbCfg.Name, err)
			} else {
				log.Printf("auto-discovery complete for %q", dbCfg.Name)
			}
		}
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
