// Package engine provides framework-agnostic business logic for querying
// and exploring PostgreSQL databases. It is used by both the CLI and the
// MCP server.
package engine

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/peter-trerotola/goro-pg/internal/config"
	"github.com/peter-trerotola/goro-pg/internal/guard"
	"github.com/peter-trerotola/goro-pg/internal/knowledgemap"
	"github.com/peter-trerotola/goro-pg/internal/postgres"
)

// Engine orchestrates read-only queries, schema discovery, and knowledge
// map lookups. It holds references to configuration, connection pools, and
// the SQLite schema cache.
type Engine struct {
	Cfg   *config.Config
	Pools *postgres.PoolManager
	Store *knowledgemap.Store
}

// New creates an Engine from a loaded config. It opens the knowledge map
// store but does NOT connect to databases — call Connect for that.
func New(cfg *config.Config) (*Engine, error) {
	store, err := knowledgemap.Open(cfg.KnowledgeMap.Path)
	if err != nil {
		return nil, fmt.Errorf("opening knowledge map: %w", err)
	}
	return &Engine{
		Cfg:   cfg,
		Pools: postgres.NewPoolManager(),
		Store: store,
	}, nil
}

// Connect establishes connection pools for all configured databases.
func (e *Engine) Connect(ctx context.Context) error {
	for _, dbCfg := range e.Cfg.Databases {
		log.Printf("connecting to database %q at %s:%d/%s", dbCfg.Name, dbCfg.Host, dbCfg.Port, dbCfg.Database)
		if err := e.Pools.Connect(ctx, dbCfg); err != nil {
			return fmt.Errorf("connecting to %q: %w", dbCfg.Name, err)
		}
		log.Printf("connected to database %q", dbCfg.Name)
	}
	return nil
}

// Shutdown closes all connection pools and the knowledge map store.
func (e *Engine) Shutdown() {
	if e.Pools != nil {
		e.Pools.Close()
	}
	if e.Store != nil {
		e.Store.Close()
	}
}

// --- Query ---

// EnrichedError wraps the original error with schema hints while
// preserving the original error for errors.Is/As.
type EnrichedError struct {
	Orig    error
	Message string
}

func (e *EnrichedError) Error() string { return e.Message }
func (e *EnrichedError) Unwrap() error { return e.Orig }

// Query executes a read-only SQL query with table filter enforcement,
// error enrichment, and schema context population.
func (e *Engine) Query(ctx context.Context, dbName, sql string) (*postgres.QueryResult, error) {
	if err := e.CheckTableFilter(dbName, sql); err != nil {
		return nil, err
	}

	result, err := postgres.ReadOnlyQuery(ctx, e.Pools, dbName, sql)
	if err != nil {
		enriched := e.enrichErrorMsg(err, dbName, sql)
		if enriched == err.Error() {
			return nil, err // no enrichment, preserve original
		}
		return nil, &EnrichedError{Orig: err, Message: enriched}
	}

	result.SchemaContext = e.BuildSchemaContext(dbName, sql)
	return result, nil
}

// --- Discovery ---

// DiscoverResult holds the outcome of a schema discovery operation.
type DiscoverResult struct {
	DatabasesDiscovered int
	TablesFound         int
	Failures            []string
}

// Discover crawls schema for a single database and populates the knowledge map.
func (e *Engine) Discover(ctx context.Context, dbName string) (*DiscoverResult, error) {
	dbCfg, err := e.FindDBConfig(dbName)
	if err != nil {
		return nil, err
	}

	pool, err := e.Pools.Get(dbName)
	if err != nil {
		return nil, err
	}

	if err := postgres.Discover(ctx, pool, *dbCfg, e.Store); err != nil {
		return nil, err
	}

	tableCount, _ := e.Store.CountTablesForDB(dbName)
	return &DiscoverResult{
		DatabasesDiscovered: 1,
		TablesFound:         tableCount,
	}, nil
}

// DiscoverAll crawls schema for all configured databases.
func (e *Engine) DiscoverAll(ctx context.Context) *DiscoverResult {
	result := &DiscoverResult{}
	for _, dbCfg := range e.Cfg.Databases {
		pool, err := e.Pools.Get(dbCfg.Name)
		if err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("%s: %v", dbCfg.Name, err))
			continue
		}
		if err := postgres.Discover(ctx, pool, dbCfg, e.Store); err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("%s: %v", dbCfg.Name, err))
			continue
		}
		result.DatabasesDiscovered++
	}
	result.TablesFound, _ = e.Store.CountTables()
	return result
}

// --- Schema Lookups ---

// ConfigDB is a summary of a configured database that has not yet been discovered.
type ConfigDB struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Database string `json:"database"`
	Status   string `json:"status"`
}

// ListDatabases returns discovered databases, falling back to configured
// databases with a "not yet discovered" status if the knowledge map is empty.
func (e *Engine) ListDatabases() ([]knowledgemap.DatabaseRow, []ConfigDB, error) {
	dbs, err := e.Store.ListDatabases()
	if err != nil {
		return nil, nil, err
	}

	if len(dbs) == 0 {
		var list []ConfigDB
		for _, db := range e.Cfg.Databases {
			list = append(list, ConfigDB{
				Name:     db.Name,
				Host:     db.Host,
				Database: db.Database,
				Status:   "not yet discovered",
			})
		}
		return nil, list, nil
	}

	return dbs, nil, nil
}

// ListSchemas returns schemas for a database from the knowledge map.
func (e *Engine) ListSchemas(dbName string) ([]knowledgemap.SchemaRow, error) {
	return e.Store.ListSchemas(dbName)
}

// ListTables returns tables for a database/schema from the knowledge map.
func (e *Engine) ListTables(dbName, schema string) ([]knowledgemap.TableRow, error) {
	return e.Store.ListTables(dbName, schema)
}

// DescribeTable returns full detail for a table from the knowledge map.
func (e *Engine) DescribeTable(dbName, schema, table string) (*knowledgemap.TableDetail, error) {
	return e.Store.DescribeTable(dbName, schema, table)
}

// ListViews returns views for a database/schema from the knowledge map.
func (e *Engine) ListViews(dbName, schema string) ([]knowledgemap.ViewRow, error) {
	return e.Store.ListViews(dbName, schema)
}

// ListFunctions returns functions for a database/schema from the knowledge map.
func (e *Engine) ListFunctions(dbName, schema string) ([]knowledgemap.FunctionRow, error) {
	return e.Store.ListFunctions(dbName, schema)
}

// SearchSchema performs a full-text search across all schema metadata.
func (e *Engine) SearchSchema(query string) ([]knowledgemap.SearchResult, error) {
	return e.Store.SearchSchema(query)
}

// --- Helpers ---

// FindDBConfig returns the config for a named database.
func (e *Engine) FindDBConfig(name string) (*config.DatabaseConfig, error) {
	for i := range e.Cfg.Databases {
		if e.Cfg.Databases[i].Name == name {
			return &e.Cfg.Databases[i], nil
		}
	}
	return nil, fmt.Errorf("database %q not found in configuration", name)
}

// CheckTableFilter enforces schema/table filters on the SQL before execution.
func (e *Engine) CheckTableFilter(dbName, sql string) error {
	if e.Cfg == nil {
		return nil
	}
	dbCfg, err := e.FindDBConfig(dbName)
	if err != nil {
		return nil // no config = no filters to enforce
	}

	return guard.CheckTableFilter(sql, func(schema, table string) bool {
		return dbCfg.ShouldIncludeSchema(schema) && dbCfg.ShouldIncludeTable(schema, table)
	})
}

// BuildSchemaContext extracts table refs from SQL and looks up columns from the knowledge map.
func (e *Engine) BuildSchemaContext(dbName, sqlStr string) map[string][]knowledgemap.ColumnSummary {
	tableRefs := guard.ExtractTableRefs(sqlStr)
	if len(tableRefs) == 0 {
		return nil
	}

	ctx := make(map[string][]knowledgemap.ColumnSummary, len(tableRefs))
	for _, ref := range tableRefs {
		cols, err := e.Store.ListColumnsCompact(dbName, ref.Schema, ref.Table)
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

// EnrichError returns the enriched error message for an error, or the
// original message if no enrichment applies. Exported for testing.
func (e *Engine) EnrichError(err error, dbName, sqlStr string) string {
	return e.enrichErrorMsg(err, dbName, sqlStr)
}

// enrichErrorMsg appends schema hints to query errors involving unknown columns or tables.
func (e *Engine) enrichErrorMsg(err error, dbName, sqlStr string) string {
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
		cols, lookupErr := e.Store.ListColumnsCompact(dbName, ref.Schema, ref.Table)
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

// maxSchemaSummaryBytes caps the schema summary size to prevent context bloat.
const maxSchemaSummaryBytes = 50_000

// BuildSchemaSummary generates a compact text summary of all discovered schemas,
// tables, and columns from the knowledge map.
func (e *Engine) BuildSchemaSummary() string {
	dbs, err := e.Store.ListDatabases()
	if err != nil || len(dbs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Schema:")
	truncated := false

	for _, db := range dbs {
		if truncated {
			break
		}
		schemas, err := e.Store.ListSchemas(db.Name)
		if err != nil {
			continue
		}
		for _, schema := range schemas {
			if truncated {
				break
			}
			tables, err := e.Store.ListTables(db.Name, schema.SchemaName)
			if err != nil {
				continue
			}
			for _, table := range tables {
				cols, err := e.Store.ListColumnsCompact(db.Name, schema.SchemaName, table.TableName)
				if err != nil {
					continue
				}
				parts := make([]string, len(cols))
				for i, c := range cols {
					parts[i] = c.Column + " (" + c.Type + ")"
				}
				line := "\n[" + db.Name + "] " + schema.SchemaName + "." + table.TableName + ": " + strings.Join(parts, ", ")
				if b.Len()+len(line) > maxSchemaSummaryBytes {
					truncated = true
					break
				}
				b.WriteString(line)
			}
		}
	}

	if b.Len() <= len("Schema:") {
		return ""
	}
	if truncated {
		b.WriteString("\n... (truncated — use describe_table for full details)")
	}
	return b.String()
}
