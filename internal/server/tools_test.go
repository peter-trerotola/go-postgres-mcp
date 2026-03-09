package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/peter-trerotola/go-postgres-mcp/internal/config"
	"github.com/peter-trerotola/go-postgres-mcp/internal/knowledgemap"
	"github.com/peter-trerotola/go-postgres-mcp/internal/postgres"
)

func TestRequireStringArg_Present(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"database": "testdb"}

	val, errResult := requireStringArg(req, "database")
	if errResult != nil {
		t.Fatal("expected no error")
	}
	if val != "testdb" {
		t.Errorf("expected %q, got %q", "testdb", val)
	}
}

func TestRequireStringArg_Missing(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	_, errResult := requireStringArg(req, "database")
	if errResult == nil {
		t.Fatal("expected error for missing argument")
	}
}

func TestRequireStringArg_WrongType(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"database": 123}

	_, errResult := requireStringArg(req, "database")
	if errResult == nil {
		t.Fatal("expected error for wrong type")
	}
}

func TestRequireStringArg_EmptyString(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"database": ""}

	_, errResult := requireStringArg(req, "database")
	if errResult == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestRequireStringArg_NilArguments(t *testing.T) {
	req := mcp.CallToolRequest{}
	// Arguments is nil (default)

	_, errResult := requireStringArg(req, "database")
	if errResult == nil {
		t.Fatal("expected error for nil arguments")
	}
}

func TestMarshalResult_ValidInput(t *testing.T) {
	input := map[string]string{"key": "value"}
	result, err := marshalResult(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestMarshalResult_NilSlice(t *testing.T) {
	var input []string
	result, err := marshalResult(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestToolDefinitions(t *testing.T) {
	tools := []struct {
		name string
		fn   func() mcp.Tool
	}{
		{"query", queryTool},
		{"discover", discoverTool},
		{"list_databases", listDatabasesTool},
		{"list_schemas", listSchemasTool},
		{"list_tables", listTablesTool},
		{"describe_table", describeTableTool},
		{"list_views", listViewsTool},
		{"list_functions", listFunctionsTool},
		{"search_schema", searchSchemaTool},
	}

	for _, tc := range tools {
		t.Run(tc.name, func(t *testing.T) {
			tool := tc.fn()
			if tool.Name != tc.name {
				t.Errorf("expected tool name %q, got %q", tc.name, tool.Name)
			}
		})
	}
}

func TestToolAnnotations_ReadOnly(t *testing.T) {
	readOnlyTools := []struct {
		name string
		fn   func() mcp.Tool
	}{
		{"query", queryTool},
		{"list_databases", listDatabasesTool},
		{"list_schemas", listSchemasTool},
		{"list_tables", listTablesTool},
		{"describe_table", describeTableTool},
		{"list_views", listViewsTool},
		{"list_functions", listFunctionsTool},
		{"search_schema", searchSchemaTool},
	}

	for _, tc := range readOnlyTools {
		t.Run(tc.name, func(t *testing.T) {
			tool := tc.fn()
			if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
				t.Errorf("expected ReadOnlyHint=true for %s", tc.name)
			}
			if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
				t.Errorf("expected DestructiveHint=false for %s", tc.name)
			}
			if tool.Annotations.IdempotentHint == nil || !*tool.Annotations.IdempotentHint {
				t.Errorf("expected IdempotentHint=true for %s", tc.name)
			}
		})
	}
}

func TestToolAnnotations_DiscoverNotReadOnly(t *testing.T) {
	tool := discoverTool()
	if tool.Annotations.ReadOnlyHint == nil || *tool.Annotations.ReadOnlyHint {
		t.Error("expected discover ReadOnlyHint=false (writes to SQLite)")
	}
	if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Error("expected discover DestructiveHint=false")
	}
	if tool.Annotations.IdempotentHint == nil || !*tool.Annotations.IdempotentHint {
		t.Error("expected discover IdempotentHint=true")
	}
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	store, err := knowledgemap.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	// Seed test data
	if err := store.InsertDatabase("testdb", "localhost", 5432, "mydb"); err != nil {
		t.Fatalf("seed database: %v", err)
	}
	if err := store.InsertSchema("testdb", "public"); err != nil {
		t.Fatalf("seed schema: %v", err)
	}
	if err := store.InsertTable("testdb", knowledgemap.TableInfo{
		SchemaName: "public", TableName: "users", TableType: "BASE TABLE",
	}); err != nil {
		t.Fatalf("seed table: %v", err)
	}
	if err := store.InsertColumn("testdb", knowledgemap.ColumnInfo{
		SchemaName: "public", TableName: "users", ColumnName: "id",
		Ordinal: 1, DataType: "integer", IsNullable: false,
	}); err != nil {
		t.Fatalf("seed column id: %v", err)
	}
	if err := store.InsertColumn("testdb", knowledgemap.ColumnInfo{
		SchemaName: "public", TableName: "users", ColumnName: "name",
		Ordinal: 2, DataType: "text", IsNullable: false,
	}); err != nil {
		t.Fatalf("seed column name: %v", err)
	}

	mcpSrv := mcpserver.NewMCPServer("test", "0.0.0",
		mcpserver.WithResourceCapabilities(false, true),
	)

	return &App{store: store, mcpServer: mcpSrv}
}

func TestBuildSchemaContext_KnownTable(t *testing.T) {
	app := newTestApp(t)
	ctx := app.buildSchemaContext("testdb", "SELECT * FROM users")
	if ctx == nil {
		t.Fatal("expected non-nil schema context")
	}
	cols, ok := ctx["public.users"]
	if !ok {
		t.Fatal("expected public.users in schema context")
	}
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(cols))
	}
	if cols[0].Column != "id" || cols[0].Type != "integer" {
		t.Errorf("unexpected first column: %+v", cols[0])
	}
	if cols[1].Column != "name" || cols[1].Type != "text" {
		t.Errorf("unexpected second column: %+v", cols[1])
	}
}

func TestBuildSchemaContext_UnknownTable(t *testing.T) {
	app := newTestApp(t)
	ctx := app.buildSchemaContext("testdb", "SELECT * FROM nonexistent")
	if ctx != nil {
		t.Errorf("expected nil schema context for unknown table, got %v", ctx)
	}
}

func TestBuildSchemaContext_NoFrom(t *testing.T) {
	app := newTestApp(t)
	ctx := app.buildSchemaContext("testdb", "SELECT 1+1")
	if ctx != nil {
		t.Errorf("expected nil schema context for no-FROM query, got %v", ctx)
	}
}

func TestBuildSchemaContext_MixedKnownUnknown(t *testing.T) {
	app := newTestApp(t)
	ctx := app.buildSchemaContext("testdb", "SELECT * FROM users JOIN unknown_table ON users.id = unknown_table.uid")
	if ctx == nil {
		t.Fatal("expected non-nil schema context")
	}
	if _, ok := ctx["public.users"]; !ok {
		t.Error("expected public.users in context")
	}
	if _, ok := ctx["public.unknown_table"]; ok {
		t.Error("did not expect unknown_table in context")
	}
}

func TestEnrichError_ColumnNotExist(t *testing.T) {
	app := newTestApp(t)
	err := fmt.Errorf(`executing query: ERROR: column "bad_col" does not exist (SQLSTATE 42703)`)
	enriched := app.enrichError(err, "testdb", "SELECT bad_col FROM users")

	if !strings.Contains(enriched, "does not exist") {
		t.Error("expected original error in enriched message")
	}
	if !strings.Contains(enriched, "Schema for referenced tables:") {
		t.Error("expected schema hint header")
	}
	if !strings.Contains(enriched, "public.users:") {
		t.Error("expected public.users in schema hint")
	}
	if !strings.Contains(enriched, "id (integer)") {
		t.Error("expected column details in schema hint")
	}
}

func TestEnrichError_TableNotExist(t *testing.T) {
	app := newTestApp(t)
	err := fmt.Errorf(`executing query: ERROR: relation "bad_table" does not exist (SQLSTATE 42P01)`)
	enriched := app.enrichError(err, "testdb", "SELECT * FROM bad_table")

	// Table doesn't exist in knowledge map either, so no hint appended
	if strings.Contains(enriched, "Schema for referenced tables:") {
		t.Error("should not have schema hint when table not in knowledge map")
	}
}

func TestEnrichError_NonSchemaError(t *testing.T) {
	app := newTestApp(t)
	err := errors.New("connection refused")
	enriched := app.enrichError(err, "testdb", "SELECT * FROM users")

	if enriched != "connection refused" {
		t.Errorf("expected unchanged error message, got %q", enriched)
	}
}

func TestEnrichError_SchemaHintForKnownTable(t *testing.T) {
	app := newTestApp(t)
	// Query references a known table but the column doesn't exist
	err := fmt.Errorf(`executing query: ERROR: column "nonexistent_col" does not exist (SQLSTATE 42703)`)
	enriched := app.enrichError(err, "testdb", "SELECT nonexistent_col FROM users")

	if !strings.Contains(enriched, "id (integer)") {
		t.Error("expected id column in hint")
	}
	if !strings.Contains(enriched, "name (text)") {
		t.Error("expected name column in hint")
	}
}

func TestRegisterResources(t *testing.T) {
	app := newTestApp(t)
	app.registerResources()
	// Verify templates are registered by listing them via the MCP server.
	// If registerResources panicked or failed, we wouldn't get here.
	// The MCP server now has 2 resource templates.
}

func TestRegisterTools(t *testing.T) {
	app := newTestApp(t)
	app.registerTools()
	// Verify tools are registered without error.
}

func TestQueryResult_SchemaContextOmittedWhenNil(t *testing.T) {
	result := &postgres.QueryResult{
		Columns: []string{"x"},
		Count:   0,
	}
	data, err := marshalResult(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// SchemaContext should be omitted (omitempty)
	if strings.Contains(data.Content[0].(mcp.TextContent).Text, "schema_context") {
		t.Error("expected schema_context to be omitted when nil")
	}
}

func newTestAppWithConfig(t *testing.T, databases []config.DatabaseConfig) *App {
	t.Helper()
	app := newTestApp(t)
	app.cfg = &config.Config{Databases: databases}
	return app
}

func TestCheckTableFilter_SchemaFilterIntegration(t *testing.T) {
	app := newTestAppWithConfig(t, []config.DatabaseConfig{
		{Name: "testdb", Schemas: []string{"public"}},
	})

	// Allowed: public schema
	if err := app.checkTableFilter("testdb", "SELECT * FROM public.users"); err != nil {
		t.Errorf("expected allowed, got: %v", err)
	}

	// Blocked: secret schema
	if err := app.checkTableFilter("testdb", "SELECT * FROM secret.data"); err == nil {
		t.Error("expected blocked for secret schema")
	}
}

func TestCheckTableFilter_TableIncludeIntegration(t *testing.T) {
	app := newTestAppWithConfig(t, []config.DatabaseConfig{
		{Name: "testdb", Tables: config.TableFilter{Include: []string{"public.users", "public.orders"}}},
	})

	// Allowed: included table
	if err := app.checkTableFilter("testdb", "SELECT * FROM users"); err != nil {
		t.Errorf("expected allowed, got: %v", err)
	}

	// Allowed: join of included tables
	if err := app.checkTableFilter("testdb", "SELECT * FROM users JOIN orders ON users.id = orders.user_id"); err != nil {
		t.Errorf("expected allowed, got: %v", err)
	}

	// Blocked: not included
	if err := app.checkTableFilter("testdb", "SELECT * FROM secrets"); err == nil {
		t.Error("expected blocked for non-included table")
	}

	// Blocked: subquery not included
	if err := app.checkTableFilter("testdb", "SELECT * FROM users WHERE id IN (SELECT user_id FROM secrets)"); err == nil {
		t.Error("expected blocked for subquery referencing non-included table")
	}
}

func TestCheckTableFilter_TableExcludeIntegration(t *testing.T) {
	app := newTestAppWithConfig(t, []config.DatabaseConfig{
		{Name: "testdb", Tables: config.TableFilter{Exclude: []string{"public.secrets"}}},
	})

	// Allowed: non-excluded
	if err := app.checkTableFilter("testdb", "SELECT * FROM users"); err != nil {
		t.Errorf("expected allowed, got: %v", err)
	}

	// Blocked: excluded
	if err := app.checkTableFilter("testdb", "SELECT * FROM secrets"); err == nil {
		t.Error("expected blocked for excluded table")
	}
}

func TestCheckTableFilter_NoConfigAllowsAll(t *testing.T) {
	app := newTestAppWithConfig(t, []config.DatabaseConfig{
		{Name: "testdb"}, // no filters
	})

	if err := app.checkTableFilter("testdb", "SELECT * FROM anything"); err != nil {
		t.Errorf("expected allowed with no filters, got: %v", err)
	}
}

func TestCheckTableFilter_UnknownDBAllowsAll(t *testing.T) {
	app := newTestAppWithConfig(t, []config.DatabaseConfig{
		{Name: "otherdb", Schemas: []string{"public"}},
	})

	// Unknown database should not enforce filters
	if err := app.checkTableFilter("testdb", "SELECT * FROM secret.data"); err != nil {
		t.Errorf("expected allowed for unknown database, got: %v", err)
	}
}

func TestCheckTableFilter_NilConfigAllowsAll(t *testing.T) {
	app := newTestApp(t) // no cfg set
	if err := app.checkTableFilter("testdb", "SELECT * FROM anything"); err != nil {
		t.Errorf("expected allowed with nil config, got: %v", err)
	}
}

// --- Schema Summary / Dynamic Instructions Tests ---

func TestBuildSchemaSummary_WithData(t *testing.T) {
	app := newTestApp(t)
	summary := app.buildSchemaSummary()

	if summary == "" {
		t.Fatal("expected non-empty schema summary")
	}
	if !strings.Contains(summary, "Schema:") {
		t.Error("expected 'Schema:' header")
	}
	if !strings.Contains(summary, "[testdb] public.users:") {
		t.Error("expected '[testdb] public.users:' line")
	}
	if !strings.Contains(summary, "id (integer)") {
		t.Error("expected 'id (integer)' in summary")
	}
	if !strings.Contains(summary, "name (text)") {
		t.Error("expected 'name (text)' in summary")
	}
}

func TestBuildSchemaSummary_EmptyStore(t *testing.T) {
	store, err := knowledgemap.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	app := &App{
		store:     store,
		mcpServer: mcpserver.NewMCPServer("test", "0.0.0"),
	}
	summary := app.buildSchemaSummary()
	if summary != "" {
		t.Errorf("expected empty summary for empty store, got %q", summary)
	}
}

func TestBuildSchemaSummary_MultipleTables(t *testing.T) {
	app := newTestApp(t)

	// Add a second table
	if err := app.store.InsertTable("testdb", knowledgemap.TableInfo{
		SchemaName: "public", TableName: "orders", TableType: "BASE TABLE",
	}); err != nil {
		t.Fatalf("seed table: %v", err)
	}
	if err := app.store.InsertColumn("testdb", knowledgemap.ColumnInfo{
		SchemaName: "public", TableName: "orders", ColumnName: "order_id",
		Ordinal: 1, DataType: "uuid",
	}); err != nil {
		t.Fatalf("seed column: %v", err)
	}
	if err := app.store.InsertColumn("testdb", knowledgemap.ColumnInfo{
		SchemaName: "public", TableName: "orders", ColumnName: "total",
		Ordinal: 2, DataType: "numeric",
	}); err != nil {
		t.Fatalf("seed column: %v", err)
	}

	summary := app.buildSchemaSummary()
	if !strings.Contains(summary, "[testdb] public.users:") {
		t.Error("expected users table in summary")
	}
	if !strings.Contains(summary, "[testdb] public.orders:") {
		t.Error("expected orders table in summary")
	}
	if !strings.Contains(summary, "order_id (uuid)") {
		t.Error("expected order_id column in summary")
	}
	if !strings.Contains(summary, "total (numeric)") {
		t.Error("expected total column in summary")
	}
}

func TestBuildSchemaSummary_MultipleDBs(t *testing.T) {
	app := newTestApp(t)

	// Add a second database
	if err := app.store.InsertDatabase("analytics", "localhost", 5433, "analytics_db"); err != nil {
		t.Fatalf("seed database: %v", err)
	}
	if err := app.store.InsertSchema("analytics", "reporting"); err != nil {
		t.Fatalf("seed schema: %v", err)
	}
	if err := app.store.InsertTable("analytics", knowledgemap.TableInfo{
		SchemaName: "reporting", TableName: "events", TableType: "BASE TABLE",
	}); err != nil {
		t.Fatalf("seed table: %v", err)
	}
	if err := app.store.InsertColumn("analytics", knowledgemap.ColumnInfo{
		SchemaName: "reporting", TableName: "events", ColumnName: "event_type",
		Ordinal: 1, DataType: "text",
	}); err != nil {
		t.Fatalf("seed column: %v", err)
	}

	summary := app.buildSchemaSummary()
	if !strings.Contains(summary, "[analytics] reporting.events:") {
		t.Error("expected analytics database in summary")
	}
	if !strings.Contains(summary, "[testdb] public.users:") {
		t.Error("expected testdb in summary")
	}
}

func TestRefreshInstructions_UpdatesMCPServer(t *testing.T) {
	app := newTestApp(t)
	app.refreshInstructions()

	// Verify instructions were updated by sending an initialize request
	initReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      mcp.NewRequestId(int64(1)),
		Request: mcp.Request{Method: "initialize"},
	}
	reqBytes, err := json.Marshal(initReq)
	if err != nil {
		t.Fatalf("marshal init request: %v", err)
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

	if !strings.Contains(initResult.Instructions, baseInstructions) {
		t.Error("expected base instructions in MCP server instructions")
	}
	if !strings.Contains(initResult.Instructions, "[testdb] public.users:") {
		t.Error("expected schema summary in MCP server instructions")
	}
	if !strings.Contains(initResult.Instructions, "id (integer)") {
		t.Error("expected column details in MCP server instructions")
	}
}

func TestRefreshInstructions_EmptyStoreKeepsBaseOnly(t *testing.T) {
	store, err := knowledgemap.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	mcpSrv := mcpserver.NewMCPServer("test", "0.0.0")
	app := &App{store: store, mcpServer: mcpSrv}
	app.refreshInstructions()

	// Verify instructions are just the base (no schema appendix)
	initReq := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      mcp.NewRequestId(int64(1)),
		Request: mcp.Request{Method: "initialize"},
	}
	reqBytes, err := json.Marshal(initReq)
	if err != nil {
		t.Fatalf("marshal init request: %v", err)
	}

	response := mcpSrv.HandleMessage(context.Background(), reqBytes)
	resp, ok := response.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", response)
	}
	initResult, ok := resp.Result.(mcp.InitializeResult)
	if !ok {
		t.Fatalf("expected InitializeResult, got %T", resp.Result)
	}

	if initResult.Instructions != baseInstructions {
		t.Errorf("expected only base instructions, got %q", initResult.Instructions)
	}
}
