package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func (a *App) registerTools() {
	a.mcpServer.AddTool(queryTool(), a.handleQuery)
	a.mcpServer.AddTool(discoverTool(), a.handleDiscover)
	a.mcpServer.AddTool(listDatabasesTool(), a.handleListDatabases)
	a.mcpServer.AddTool(listSchemasTool(), a.handleListSchemas)
	a.mcpServer.AddTool(listTablesTool(), a.handleListTables)
	a.mcpServer.AddTool(describeTableTool(), a.handleDescribeTable)
	a.mcpServer.AddTool(listViewsTool(), a.handleListViews)
	a.mcpServer.AddTool(listFunctionsTool(), a.handleListFunctions)
	a.mcpServer.AddTool(searchSchemaTool(), a.handleSearchSchema)
}

// --- Tool Definitions ---

func queryTool() mcp.Tool {
	return mcp.NewTool("query",
		mcp.WithDescription("Execute a read-only SELECT query against a PostgreSQL database. Only SELECT statements are allowed."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database")),
		mcp.WithString("sql", mcp.Required(), mcp.Description("SQL SELECT query to execute")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func discoverTool() mcp.Tool {
	return mcp.NewTool("discover",
		mcp.WithDescription("Discover or refresh schema for configured databases. Crawls tables, columns, constraints, indexes, views, and functions. If database is omitted, discovers all configured databases."),
		mcp.WithString("database", mcp.Description("Name of a specific database to discover (omit to discover all)")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func listDatabasesTool() mcp.Tool {
	return mcp.NewTool("list_databases",
		mcp.WithDescription("List all configured databases from the knowledge map."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func listSchemasTool() mcp.Tool {
	return mcp.NewTool("list_schemas",
		mcp.WithDescription("List schemas in a database from the knowledge map."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func listTablesTool() mcp.Tool {
	return mcp.NewTool("list_tables",
		mcp.WithDescription("List tables in a schema from the knowledge map."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database")),
		mcp.WithString("schema", mcp.Required(), mcp.Description("Schema name (e.g. 'public')")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func describeTableTool() mcp.Tool {
	return mcp.NewTool("describe_table",
		mcp.WithDescription("Get full detail for a table including columns, constraints, indexes, and foreign keys."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database")),
		mcp.WithString("schema", mcp.Required(), mcp.Description("Schema name")),
		mcp.WithString("table", mcp.Required(), mcp.Description("Table name")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func listViewsTool() mcp.Tool {
	return mcp.NewTool("list_views",
		mcp.WithDescription("List views in a schema from the knowledge map."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database")),
		mcp.WithString("schema", mcp.Required(), mcp.Description("Schema name")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func listFunctionsTool() mcp.Tool {
	return mcp.NewTool("list_functions",
		mcp.WithDescription("List functions in a schema from the knowledge map."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database")),
		mcp.WithString("schema", mcp.Required(), mcp.Description("Schema name")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

func searchSchemaTool() mcp.Tool {
	return mcp.NewTool("search_schema",
		mcp.WithDescription("Full-text search across all schema metadata in the knowledge map."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query keywords")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
}

// --- Argument Helpers ---

// requireStringArg safely extracts a string argument from the request,
// returning a tool error if the argument is missing or not a string.
func requireStringArg(request mcp.CallToolRequest, key string) (string, *mcp.CallToolResult) {
	val, ok := request.GetArguments()[key].(string)
	if !ok || val == "" {
		return "", mcp.NewToolResultError(fmt.Sprintf("missing required argument: %s", key))
	}
	return val, nil
}

// marshalResult marshals a value to indented JSON, returning a tool error on failure.
func marshalResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshaling result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// --- Handlers ---

func (a *App) handleQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dbName, errResult := requireStringArg(request, "database")
	if errResult != nil {
		return errResult, nil
	}
	sqlStr, errResult := requireStringArg(request, "sql")
	if errResult != nil {
		return errResult, nil
	}

	result, err := a.engine.Query(ctx, dbName, sqlStr)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return marshalResult(result)
}

func (a *App) handleDiscover(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	if raw, ok := args["database"]; ok {
		s, isStr := raw.(string)
		if !isStr || s == "" {
			return mcp.NewToolResultError("database must be a non-empty string"), nil
		}
		a.sendLog(mcp.LoggingLevelInfo, fmt.Sprintf("discovering schema for %s...", s))
		dr, err := a.engine.Discover(ctx, s)
		if err != nil {
			a.sendLog(mcp.LoggingLevelWarning, fmt.Sprintf("schema discovery failed for %s: %v", s, err))
			return mcp.NewToolResultError(err.Error()), nil
		}
		a.sendLog(mcp.LoggingLevelInfo, fmt.Sprintf("ready — %d tables discovered for %s", dr.TablesFound, s))
		a.refreshInstructions()
		return mcp.NewToolResultText(fmt.Sprintf("Successfully discovered schema for database %q", s)), nil
	}

	// Discover all databases via engine
	dr := a.engine.DiscoverAll(ctx)

	if len(dr.Failures) > 0 {
		a.sendLog(mcp.LoggingLevelWarning, fmt.Sprintf("partial discovery — %d tables across %d databases (%d failed)", dr.TablesFound, dr.DatabasesDiscovered, len(dr.Failures)))
	} else {
		a.sendLog(mcp.LoggingLevelInfo, fmt.Sprintf("ready — %d tables across %d databases", dr.TablesFound, dr.DatabasesDiscovered))
	}
	a.refreshInstructions()

	if len(dr.Failures) > 0 {
		return mcp.NewToolResultError(fmt.Sprintf("discovery failed for: %s", strings.Join(dr.Failures, "; "))), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully discovered schema for %d databases", dr.DatabasesDiscovered)), nil
}

func (a *App) handleListDatabases(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dbs, configDBs, err := a.engine.ListDatabases()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if configDBs != nil {
		return marshalResult(configDBs)
	}
	return marshalResult(dbs)
}

func (a *App) handleListSchemas(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dbName, errResult := requireStringArg(request, "database")
	if errResult != nil {
		return errResult, nil
	}
	schemas, err := a.engine.ListSchemas(dbName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return marshalResult(schemas)
}

func (a *App) handleListTables(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dbName, errResult := requireStringArg(request, "database")
	if errResult != nil {
		return errResult, nil
	}
	schemaName, errResult := requireStringArg(request, "schema")
	if errResult != nil {
		return errResult, nil
	}
	tables, err := a.engine.ListTables(dbName, schemaName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return marshalResult(tables)
}

func (a *App) handleDescribeTable(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dbName, errResult := requireStringArg(request, "database")
	if errResult != nil {
		return errResult, nil
	}
	schemaName, errResult := requireStringArg(request, "schema")
	if errResult != nil {
		return errResult, nil
	}
	tableName, errResult := requireStringArg(request, "table")
	if errResult != nil {
		return errResult, nil
	}
	detail, err := a.engine.DescribeTable(dbName, schemaName, tableName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return marshalResult(detail)
}

func (a *App) handleListViews(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dbName, errResult := requireStringArg(request, "database")
	if errResult != nil {
		return errResult, nil
	}
	schemaName, errResult := requireStringArg(request, "schema")
	if errResult != nil {
		return errResult, nil
	}
	views, err := a.engine.ListViews(dbName, schemaName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return marshalResult(views)
}

func (a *App) handleListFunctions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dbName, errResult := requireStringArg(request, "database")
	if errResult != nil {
		return errResult, nil
	}
	schemaName, errResult := requireStringArg(request, "schema")
	if errResult != nil {
		return errResult, nil
	}
	functions, err := a.engine.ListFunctions(dbName, schemaName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return marshalResult(functions)
}

func (a *App) handleSearchSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, errResult := requireStringArg(request, "query")
	if errResult != nil {
		return errResult, nil
	}
	results, err := a.engine.SearchSchema(query)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if len(results) == 0 {
		return mcp.NewToolResultText("No results found"), nil
	}
	return marshalResult(results)
}
