package server

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/peter-trerotola/goro-pg/internal/config"
	"github.com/peter-trerotola/goro-pg/internal/engine"
	"github.com/peter-trerotola/goro-pg/internal/knowledgemap"
	"github.com/peter-trerotola/goro-pg/internal/postgres"
)

func TestNew_LoggingCapabilityInInitialize(t *testing.T) {
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

	// Send an initialize request and verify logging capability is advertised
	initReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      mcp.NewRequestId(int64(1)),
		Request: mcp.Request{Method: "initialize"},
	}
	reqBytes, err := json.Marshal(initReq)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	response := app.mcpServer.HandleMessage(context.Background(), reqBytes)
	resp, ok := response.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", response)
	}
	initResult, ok := resp.Result.(mcp.InitializeResult)
	if !ok {
		t.Fatalf("expected InitializeResult, got %T", resp.Result)
	}

	if initResult.Capabilities.Logging == nil {
		t.Error("expected logging capability to be advertised in initialize response")
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

	app := &App{
		engine:    &engine.Engine{Store: store},
		mcpServer: mcpSrv,
	}

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
		engine: &engine.Engine{
			Cfg: &config.Config{
				KnowledgeMap: config.KnowledgeMapConfig{
					AutoDiscoverOnStartup: false,
				},
			},
			Store: store,
		},
		mcpServer: mcpSrv,
	}

	// Should return immediately without starting discovery
	app.onAfterInitialize(nil, nil, &mcp.InitializeRequest{}, &mcp.InitializeResult{})
	// If it tried to discover without pools, it would panic - no panic = success
}

func TestOnAfterInitialize_OnlyRunsOnce(t *testing.T) {
	store, err := knowledgemap.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	mcpSrv := mcpserver.NewMCPServer("test", "0.0.0",
		mcpserver.WithLogging(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := &App{
		engine: &engine.Engine{
			Cfg: &config.Config{
				KnowledgeMap: config.KnowledgeMapConfig{
					AutoDiscoverOnStartup: true,
				},
			},
			Store: store,
			Pools: postgres.NewPoolManager(), // empty — runAutoDiscovery will skip all DBs
		},
		mcpServer:      mcpSrv,
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
	}

	// Call multiple times — sync.Once should ensure only one goroutine launches
	for i := 0; i < 5; i++ {
		app.onAfterInitialize(nil, nil, &mcp.InitializeRequest{}, &mcp.InitializeResult{})
	}
	// No panic or race = success (run with -race)
}

func TestShutdown_CancelsContext(t *testing.T) {
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

	// Verify context is not cancelled before shutdown
	select {
	case <-app.shutdownCtx.Done():
		t.Fatal("expected context to not be cancelled before shutdown")
	default:
	}

	app.Shutdown()

	// Verify context is cancelled after shutdown
	select {
	case <-app.shutdownCtx.Done():
		// expected
	default:
		t.Fatal("expected context to be cancelled after shutdown")
	}
}
