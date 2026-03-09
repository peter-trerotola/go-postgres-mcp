package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/peter-trerotola/go-postgres-mcp/internal/config"
	"github.com/peter-trerotola/go-postgres-mcp/internal/guard"
	"github.com/peter-trerotola/go-postgres-mcp/internal/knowledgemap"
	"github.com/peter-trerotola/go-postgres-mcp/internal/postgres"
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

	// Enforce table filters before executing
	if err := a.checkTableFilter(dbName, sqlStr); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result, err := postgres.ReadOnlyQuery(ctx, a.pools, dbName, sqlStr)
	if err != nil {
		return mcp.NewToolResultError(a.enrichError(err, dbName, sqlStr)), nil
	}

	// Populate schema context from knowledge map
	result.SchemaContext = a.buildSchemaContext(dbName, sqlStr)

	return marshalResult(result)
}

// buildSchemaContext extracts table refs from SQL and looks up columns from the knowledge map.
// Note: SQL is parsed again here after guard.Validate in ReadOnlyQuery. The double parse
// is intentional — ReadOnlyQuery validates internally as defense-in-depth, and pg_query
// parsing is sub-millisecond. The Postgres round-trip dominates query latency.
func (a *App) buildSchemaContext(dbName, sqlStr string) map[string][]knowledgemap.ColumnSummary {
	tableRefs := guard.ExtractTableRefs(sqlStr)
	if len(tableRefs) == 0 {
		return nil
	}

	ctx := make(map[string][]knowledgemap.ColumnSummary, len(tableRefs))
	for _, ref := range tableRefs {
		cols, err := a.store.ListColumnsCompact(dbName, ref.Schema, ref.Table)
		if err != nil || len(cols) == 0 {
			continue
		}
		key := ref.Schema + "." + ref.Table
		ctx[key] = cols
	}

	if len(ctx) == 0 {
		return nil
	}
	return ctx
}

// enrichError appends schema hints to query errors involving unknown columns or tables.
func (a *App) enrichError(err error, dbName, sqlStr string) string {
	msg := err.Error()

	if !strings.Contains(msg, "does not exist") {
		return msg
	}

	tableRefs := guard.ExtractTableRefs(sqlStr)
	if len(tableRefs) == 0 {
		return msg
	}

	var hints []string
	for _, ref := range tableRefs {
		cols, lookupErr := a.store.ListColumnsCompact(dbName, ref.Schema, ref.Table)
		if lookupErr != nil || len(cols) == 0 {
			continue
		}
		parts := make([]string, len(cols))
		for i, c := range cols {
			parts[i] = c.Column + " (" + c.Type + ")"
		}
		hints = append(hints, fmt.Sprintf("  %s.%s: %s", ref.Schema, ref.Table, strings.Join(parts, ", ")))
	}

	if len(hints) == 0 {
		return msg
	}

	return msg + "\n\nSchema for referenced tables:\n" + strings.Join(hints, "\n")
}

func (a *App) handleDiscover(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// If a specific database is provided, discover only that one.
	// Otherwise, discover all configured databases.
	dbName, _ := request.GetArguments()["database"].(string)

	var targets []config.DatabaseConfig
	if dbName != "" {
		dbCfg, err := a.findDBConfig(dbName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		targets = []config.DatabaseConfig{*dbCfg}
	} else {
		targets = a.cfg.Databases
	}

	var failed []string
	for _, dbCfg := range targets {
		pool, err := a.pools.Get(dbCfg.Name)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", dbCfg.Name, err))
			continue
		}
		a.sendLog(mcp.LoggingLevelInfo, fmt.Sprintf("discovering schema for %s...", dbCfg.Name))
		if err := postgres.Discover(ctx, pool, dbCfg, a.store); err != nil {
			a.sendLog(mcp.LoggingLevelWarning, fmt.Sprintf("schema discovery failed for %s: %v", dbCfg.Name, err))
			failed = append(failed, fmt.Sprintf("%s: %v", dbCfg.Name, err))
		}
	}

	tableCount, err := a.store.CountTables()
	if err != nil {
		a.sendLog(mcp.LoggingLevelWarning, "schema discovery complete but failed to count tables")
	} else {
		dbs, dbErr := a.store.ListDatabases()
		dbCount := len(a.cfg.Databases)
		if dbErr == nil {
			dbCount = len(dbs)
		}
		a.sendLog(mcp.LoggingLevelInfo, fmt.Sprintf("ready — %d tables across %d databases", tableCount, dbCount))
	}
	a.refreshInstructions()

	if len(failed) > 0 {
		return mcp.NewToolResultError(fmt.Sprintf("discovery failed for: %s", strings.Join(failed, "; "))), nil
	}
	if dbName != "" {
		return mcp.NewToolResultText(fmt.Sprintf("Successfully discovered schema for database %q", dbName)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully discovered schema for %d databases", len(targets))), nil
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

// checkTableFilter enforces schema/table filters on the SQL before execution.
// If the database has filters configured, all table references in the SQL are
// validated against ShouldIncludeSchema and ShouldIncludeTable.
func (a *App) checkTableFilter(dbName, sql string) error {
	if a.cfg == nil {
		return nil
	}
	dbCfg, err := a.findDBConfig(dbName)
	if err != nil {
		return nil // no config = no filters to enforce
	}

	return guard.CheckTableFilter(sql, func(schema, table string) bool {
		return dbCfg.ShouldIncludeSchema(schema) && dbCfg.ShouldIncludeTable(schema, table)
	})
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
