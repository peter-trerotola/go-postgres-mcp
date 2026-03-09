package server

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/peter-trerotola/go-postgres-mcp/internal/config"
	"github.com/peter-trerotola/go-postgres-mcp/internal/knowledgemap"
)

func TestNew_RegistersLoggingCapability(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		KnowledgeMap: config.KnowledgeMapConfig{
			Path: filepath.Join(dir, "test.db"),
		},
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer app.Shutdown()

	// The server should be created successfully with logging enabled.
	// We verify by checking the MCPServer is non-nil.
	if app.MCPServer() == nil {
		t.Fatal("expected non-nil MCPServer")
	}
}

func TestNew_RegistersHooks(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		KnowledgeMap: config.KnowledgeMapConfig{
			Path: filepath.Join(dir, "test.db"),
		},
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer app.Shutdown()

	// Verify we can create the app with hooks. The OnAfterInitialize hook
	// is registered internally - we test its behavior via integration tests.
	if app.mcpServer == nil {
		t.Fatal("expected mcpServer to be initialized")
	}
}

func TestSendLog_DoesNotPanic(t *testing.T) {
	store, err := knowledgemap.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	mcpSrv := mcpserver.NewMCPServer("test", "0.0.0",
		mcpserver.WithLogging(),
	)

	app := &App{store: store, mcpServer: mcpSrv}

	// sendLog should not panic even with no connected clients
	app.sendLog(mcp.LoggingLevelInfo, "test message")
	app.sendLog(mcp.LoggingLevelWarning, "warning message")
	app.sendLog(mcp.LoggingLevelError, "error message")
}

func TestOnAfterInitialize_SkipsWhenAutoDiscoverDisabled(t *testing.T) {
	store, err := knowledgemap.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	mcpSrv := mcpserver.NewMCPServer("test", "0.0.0",
		mcpserver.WithLogging(),
	)

	app := &App{
		cfg: &config.Config{
			KnowledgeMap: config.KnowledgeMapConfig{
				AutoDiscoverOnStartup: false,
			},
		},
		store:     store,
		mcpServer: mcpSrv,
	}

	// Should return immediately without starting discovery
	app.onAfterInitialize(nil, nil, &mcp.InitializeRequest{}, &mcp.InitializeResult{})
	// If it tried to discover without pools, it would panic - no panic = success
}

func TestStartDoesNotRunDiscovery(t *testing.T) {
	// Start() should only connect to databases, not run discovery.
	// Since we have no real databases, Start with empty config should succeed.
	dir := t.TempDir()
	cfg := &config.Config{
		KnowledgeMap: config.KnowledgeMapConfig{
			Path:                  filepath.Join(dir, "test.db"),
			AutoDiscoverOnStartup: true, // Even with auto-discover enabled
		},
		Databases: []config.DatabaseConfig{}, // No databases to connect to
	}

	app, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer app.Shutdown()

	// Start with no databases should succeed immediately
	if err := app.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Verify no tables were discovered (discovery only runs in hook)
	count, err := app.store.CountTables()
	if err != nil {
		t.Fatalf("CountTables: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 tables after Start (discovery deferred to hook), got %d", count)
	}
}
