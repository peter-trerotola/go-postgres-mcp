package server

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/peter-trerotola/goro-pg/internal/knowledgemap"
)

func TestHandleResourceTables(t *testing.T) {
	app := newTestApp(t)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "schema:///testdb/tables"
	req.Params.Arguments = map[string]any{"database": "testdb"}

	contents, err := app.handleResourceTables(context.Background(), req)
	if err != nil {
		t.Fatalf("handleResourceTables: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	text := contents[0].(mcp.TextResourceContents)
	if text.MIMEType != "application/json" {
		t.Errorf("expected application/json, got %s", text.MIMEType)
	}
	if !strings.Contains(text.Text, `"users"`) {
		t.Error("expected users table in response")
	}
	if !strings.Contains(text.Text, `"column_count"`) {
		t.Error("expected column_count in response")
	}
}

func TestHandleResourceTables_EmptyDB(t *testing.T) {
	app := newTestApp(t)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "schema:///emptydb/tables"
	req.Params.Arguments = map[string]any{"database": "emptydb"}

	contents, err := app.handleResourceTables(context.Background(), req)
	if err != nil {
		t.Fatalf("handleResourceTables: %v", err)
	}
	text := contents[0].(mcp.TextResourceContents)
	if text.Text != "[]" {
		t.Errorf("expected empty array for empty DB, got %s", text.Text)
	}
}

func TestHandleResourceTables_MissingDatabase(t *testing.T) {
	app := newTestApp(t)

	req := mcp.ReadResourceRequest{}
	req.Params.Arguments = map[string]any{}

	_, err := app.handleResourceTables(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing database")
	}
}

func TestHandleResourceTableDetail(t *testing.T) {
	app := newTestApp(t)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "schema:///testdb/public/users"
	req.Params.Arguments = map[string]any{
		"database": "testdb",
		"schema":   "public",
		"table":    "users",
	}

	contents, err := app.handleResourceTableDetail(context.Background(), req)
	if err != nil {
		t.Fatalf("handleResourceTableDetail: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}

	text := contents[0].(mcp.TextResourceContents)
	if !strings.Contains(text.Text, `"column_name"`) {
		t.Error("expected column details in response")
	}
	if !strings.Contains(text.Text, `"id"`) {
		t.Error("expected id column in response")
	}
}

func TestHandleResourceTableDetail_NotFound(t *testing.T) {
	app := newTestApp(t)

	req := mcp.ReadResourceRequest{}
	req.Params.Arguments = map[string]any{
		"database": "testdb",
		"schema":   "public",
		"table":    "nonexistent",
	}

	_, err := app.handleResourceTableDetail(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nonexistent table")
	}
}

func TestHandleResourceTableDetail_MissingParams(t *testing.T) {
	app := newTestApp(t)

	req := mcp.ReadResourceRequest{}
	req.Params.Arguments = map[string]any{"database": "testdb"}

	_, err := app.handleResourceTableDetail(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing schema/table")
	}
}

func TestHandleResourceTables_MultipleSchemas(t *testing.T) {
	app := newTestApp(t)

	// Add another schema with a table
	app.engine.Store.InsertSchema("testdb", "audit")
	app.engine.Store.InsertTable("testdb", knowledgemap.TableInfo{
		SchemaName: "audit", TableName: "logs", TableType: "BASE TABLE",
	})
	app.engine.Store.InsertColumn("testdb", knowledgemap.ColumnInfo{
		SchemaName: "audit", TableName: "logs", ColumnName: "id",
		Ordinal: 1, DataType: "bigint",
	})

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "schema:///testdb/tables"
	req.Params.Arguments = map[string]any{"database": "testdb"}

	contents, err := app.handleResourceTables(context.Background(), req)
	if err != nil {
		t.Fatalf("handleResourceTables: %v", err)
	}

	text := contents[0].(mcp.TextResourceContents)
	if !strings.Contains(text.Text, `"users"`) {
		t.Error("expected users table")
	}
	if !strings.Contains(text.Text, `"logs"`) {
		t.Error("expected logs table")
	}
	if !strings.Contains(text.Text, `"audit"`) {
		t.Error("expected audit schema")
	}
}
