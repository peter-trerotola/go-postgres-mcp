package engine

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/peter-trerotola/goro-pg/internal/config"
	"github.com/peter-trerotola/goro-pg/internal/knowledgemap"
)

func newTestEngine(t *testing.T) *Engine {
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

	return &Engine{Store: store}
}

func TestNew(t *testing.T) {
	cfg := &config.Config{
		KnowledgeMap: config.KnowledgeMapConfig{
			Path: filepath.Join(t.TempDir(), "test.db"),
		},
	}
	eng, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer eng.Shutdown()

	if eng.Cfg != cfg {
		t.Error("expected config to be stored")
	}
	if eng.Pools == nil {
		t.Error("expected pools to be initialized")
	}
	if eng.Store == nil {
		t.Error("expected store to be initialized")
	}
}

func TestListDatabases_WithData(t *testing.T) {
	eng := newTestEngine(t)
	dbs, configDBs, err := eng.ListDatabases()
	if err != nil {
		t.Fatalf("ListDatabases: %v", err)
	}
	if configDBs != nil {
		t.Error("expected no config fallback when data exists")
	}
	if len(dbs) != 1 || dbs[0].Name != "testdb" {
		t.Errorf("expected 1 database 'testdb', got %v", dbs)
	}
}

func TestListDatabases_EmptyStore(t *testing.T) {
	store, err := knowledgemap.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	eng := &Engine{
		Cfg: &config.Config{
			Databases: []config.DatabaseConfig{{Name: "db1", Host: "h", Database: "d"}},
		},
		Store: store,
	}

	dbs, configDBs, err := eng.ListDatabases()
	if err != nil {
		t.Fatalf("ListDatabases: %v", err)
	}
	if dbs != nil {
		t.Error("expected nil dbs for empty store")
	}
	if len(configDBs) != 1 || configDBs[0].Status != "not yet discovered" {
		t.Errorf("expected config fallback, got %v", configDBs)
	}
}

func TestListSchemas(t *testing.T) {
	eng := newTestEngine(t)
	schemas, err := eng.ListSchemas("testdb")
	if err != nil {
		t.Fatalf("ListSchemas: %v", err)
	}
	if len(schemas) != 1 || schemas[0].SchemaName != "public" {
		t.Errorf("expected 1 schema 'public', got %v", schemas)
	}
}

func TestListTables(t *testing.T) {
	eng := newTestEngine(t)
	tables, err := eng.ListTables("testdb", "public")
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	if len(tables) != 1 || tables[0].TableName != "users" {
		t.Errorf("expected 1 table 'users', got %v", tables)
	}
}

func TestDescribeTable(t *testing.T) {
	eng := newTestEngine(t)
	detail, err := eng.DescribeTable("testdb", "public", "users")
	if err != nil {
		t.Fatalf("DescribeTable: %v", err)
	}
	if detail.Table.TableName != "users" {
		t.Errorf("expected table name 'users', got %q", detail.Table.TableName)
	}
	if len(detail.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(detail.Columns))
	}
}

func TestBuildSchemaContext(t *testing.T) {
	eng := newTestEngine(t)
	ctx := eng.BuildSchemaContext("testdb", "SELECT * FROM users")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	cols, ok := ctx["public.users"]
	if !ok {
		t.Fatal("expected public.users in context")
	}
	if len(cols) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(cols))
	}
}

func TestEnrichError_WithHint(t *testing.T) {
	eng := newTestEngine(t)
	err := &testError{msg: `column "bad" does not exist`}
	enriched := eng.EnrichError(err, "testdb", "SELECT bad FROM users")
	if !strings.Contains(enriched, "Schema for referenced tables:") {
		t.Error("expected schema hint")
	}
	if !strings.Contains(enriched, "id (integer)") {
		t.Error("expected column detail")
	}
}

func TestEnrichError_NoHint(t *testing.T) {
	eng := newTestEngine(t)
	err := &testError{msg: "connection refused"}
	enriched := eng.EnrichError(err, "testdb", "SELECT * FROM users")
	if enriched != "connection refused" {
		t.Errorf("expected unchanged message, got %q", enriched)
	}
}

func TestCheckTableFilter_WithSchemaFilter(t *testing.T) {
	eng := newTestEngine(t)
	eng.Cfg = &config.Config{
		Databases: []config.DatabaseConfig{
			{Name: "testdb", Schemas: []string{"public"}},
		},
	}

	if err := eng.CheckTableFilter("testdb", "SELECT * FROM public.users"); err != nil {
		t.Errorf("expected allowed: %v", err)
	}
	if err := eng.CheckTableFilter("testdb", "SELECT * FROM secret.data"); err == nil {
		t.Error("expected blocked for secret schema")
	}
}

func TestCheckTableFilter_NilConfig(t *testing.T) {
	eng := newTestEngine(t)
	if err := eng.CheckTableFilter("testdb", "SELECT * FROM anything"); err != nil {
		t.Errorf("expected allowed with nil config: %v", err)
	}
}

func TestBuildSchemaSummary(t *testing.T) {
	eng := newTestEngine(t)
	summary := eng.BuildSchemaSummary()
	if !strings.Contains(summary, "Schema:") {
		t.Error("expected Schema: header")
	}
	if !strings.Contains(summary, "[testdb] public.users:") {
		t.Error("expected table line")
	}
	if !strings.Contains(summary, "id (integer)") {
		t.Error("expected column")
	}
}

func TestBuildSchemaSummary_Empty(t *testing.T) {
	store, err := knowledgemap.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	eng := &Engine{Store: store}
	if summary := eng.BuildSchemaSummary(); summary != "" {
		t.Errorf("expected empty summary, got %q", summary)
	}
}

func TestFindDBConfig(t *testing.T) {
	eng := newTestEngine(t)
	eng.Cfg = &config.Config{
		Databases: []config.DatabaseConfig{{Name: "testdb"}},
	}

	cfg, err := eng.FindDBConfig("testdb")
	if err != nil {
		t.Fatalf("FindDBConfig: %v", err)
	}
	if cfg.Name != "testdb" {
		t.Errorf("expected testdb, got %q", cfg.Name)
	}

	_, err = eng.FindDBConfig("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent database")
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
