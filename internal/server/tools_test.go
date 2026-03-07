package server

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
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
