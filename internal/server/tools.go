package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/petros/go-postgres-mcp/internal/config"
	"github.com/petros/go-postgres-mcp/internal/postgres"
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
	)
}

func discoverTool() mcp.Tool {
	return mcp.NewTool("discover",
		mcp.WithDescription("Discover or refresh the schema for a configured database. Crawls tables, columns, constraints, indexes, views, and functions."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database to discover")),
	)
}

func listDatabasesTool() mcp.Tool {
	return mcp.NewTool("list_databases",
		mcp.WithDescription("List all configured databases from the knowledge map."),
	)
}

func listSchemasTool() mcp.Tool {
	return mcp.NewTool("list_schemas",
		mcp.WithDescription("List schemas in a database from the knowledge map."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database")),
	)
}

func listTablesTool() mcp.Tool {
	return mcp.NewTool("list_tables",
		mcp.WithDescription("List tables in a schema from the knowledge map."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database")),
		mcp.WithString("schema", mcp.Required(), mcp.Description("Schema name (e.g. 'public')")),
	)
}

func describeTableTool() mcp.Tool {
	return mcp.NewTool("describe_table",
		mcp.WithDescription("Get full detail for a table including columns, constraints, indexes, and foreign keys."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database")),
		mcp.WithString("schema", mcp.Required(), mcp.Description("Schema name")),
		mcp.WithString("table", mcp.Required(), mcp.Description("Table name")),
	)
}

func listViewsTool() mcp.Tool {
	return mcp.NewTool("list_views",
		mcp.WithDescription("List views in a schema from the knowledge map."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database")),
		mcp.WithString("schema", mcp.Required(), mcp.Description("Schema name")),
	)
}

func listFunctionsTool() mcp.Tool {
	return mcp.NewTool("list_functions",
		mcp.WithDescription("List functions in a schema from the knowledge map."),
		mcp.WithString("database", mcp.Required(), mcp.Description("Name of the configured database")),
		mcp.WithString("schema", mcp.Required(), mcp.Description("Schema name")),
	)
}

func searchSchemaTool() mcp.Tool {
	return mcp.NewTool("search_schema",
		mcp.WithDescription("Full-text search across all schema metadata in the knowledge map."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query keywords")),
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

	result, err := postgres.ReadOnlyQuery(ctx, a.pools, dbName, sqlStr)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return marshalResult(result)
}

func (a *App) handleDiscover(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dbName, errResult := requireStringArg(request, "database")
	if errResult != nil {
		return errResult, nil
	}

	dbCfg, err := a.findDBConfig(dbName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pool, err := a.pools.Get(dbName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := postgres.Discover(ctx, pool, *dbCfg, a.store); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("discovery failed: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully discovered schema for database %q", dbName)), nil
}

func (a *App) handleListDatabases(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dbs, err := a.store.ListDatabases()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if len(dbs) == 0 {
		type configDB struct {
			Name     string `json:"name"`
			Host     string `json:"host"`
			Database string `json:"database"`
			Status   string `json:"status"`
		}
		var list []configDB
		for _, db := range a.cfg.Databases {
			list = append(list, configDB{
				Name:     db.Name,
				Host:     db.Host,
				Database: db.Database,
				Status:   "not yet discovered",
			})
		}
		return marshalResult(list)
	}

	return marshalResult(dbs)
}

func (a *App) handleListSchemas(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dbName, errResult := requireStringArg(request, "database")
	if errResult != nil {
		return errResult, nil
	}
	schemas, err := a.store.ListSchemas(dbName)
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
	tables, err := a.store.ListTables(dbName, schemaName)
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
	detail, err := a.store.DescribeTable(dbName, schemaName, tableName)
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
	views, err := a.store.ListViews(dbName, schemaName)
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
	functions, err := a.store.ListFunctions(dbName, schemaName)
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
	results, err := a.store.SearchSchema(query)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if len(results) == 0 {
		return mcp.NewToolResultText("No results found"), nil
	}
	return marshalResult(results)
}

// findDBConfig returns the config for a named database.
func (a *App) findDBConfig(name string) (*config.DatabaseConfig, error) {
	for i := range a.cfg.Databases {
		if a.cfg.Databases[i].Name == name {
			return &a.cfg.Databases[i], nil
		}
	}
	return nil, fmt.Errorf("database %q not found in configuration", name)
}
