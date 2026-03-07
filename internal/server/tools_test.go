package server

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
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
