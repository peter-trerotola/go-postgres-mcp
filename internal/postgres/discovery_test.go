package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peter-trerotola/goro-pg/internal/config"
	"github.com/peter-trerotola/goro-pg/internal/knowledgemap"
)

// Integration tests require a running PostgreSQL instance.
// Run: INTEGRATION_TEST=1 go test ./internal/postgres/ -run TestDiscover -v
//
// The tests use the docker-compose postgres service:
//   docker compose up -d postgres
//   INTEGRATION_TEST=1 go test ./internal/postgres/ -run TestDiscover -v -count=1

func skipUnlessIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test; set INTEGRATION_TEST=1 to run")
	}
}

// setupTestDB connects as the superuser, creates a restricted user and
// schemas with varying access, then returns a pool connected as the
// restricted user. Cleanup is handled via t.Cleanup.
func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	// Connect as superuser
	port := envOr("POSTGRES_PORT", "5432")
	adminURL := fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable",
		envOr("POSTGRES_USER", "testuser"),
		envOr("POSTGRES_PASSWORD", "testpass"),
		port,
		envOr("POSTGRES_DB", "testdb"),
	)
	adminPool, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatalf("connecting as admin: %v", err)
	}
	t.Cleanup(func() { adminPool.Close() })

	// Create test schemas: "visible" (user has access) and "hidden" (user does not)
	setup := []string{
		`DROP SCHEMA IF EXISTS test_visible CASCADE`,
		`DROP SCHEMA IF EXISTS test_hidden CASCADE`,
		`DROP ROLE IF EXISTS test_restricted_user`,
		`CREATE ROLE test_restricted_user LOGIN PASSWORD 'restricted_pass'`,
		// Visible schema — user gets USAGE + SELECT
		`CREATE SCHEMA test_visible`,
		`CREATE TABLE test_visible.users (id serial PRIMARY KEY, name text NOT NULL)`,
		`CREATE TABLE test_visible.orders (id serial PRIMARY KEY, user_id int REFERENCES test_visible.users(id), total numeric)`,
		`CREATE INDEX idx_orders_user_id ON test_visible.orders(user_id)`,
		`CREATE VIEW test_visible.user_orders AS SELECT u.name, o.total FROM test_visible.users u JOIN test_visible.orders o ON o.user_id = u.id`,
		`CREATE FUNCTION test_visible.add(a int, b int) RETURNS int LANGUAGE sql AS 'SELECT a + b'`,
		`GRANT USAGE ON SCHEMA test_visible TO test_restricted_user`,
		`GRANT SELECT ON ALL TABLES IN SCHEMA test_visible TO test_restricted_user`,
		// Hidden schema — user gets NO access
		`CREATE SCHEMA test_hidden`,
		`CREATE TABLE test_hidden.secrets (id serial PRIMARY KEY, value text NOT NULL)`,
		`CREATE TABLE test_hidden.audit_log (id serial PRIMARY KEY, secret_id int REFERENCES test_hidden.secrets(id), action text)`,
		`CREATE INDEX idx_audit_secret ON test_hidden.audit_log(secret_id)`,
		`CREATE VIEW test_hidden.secret_view AS SELECT value FROM test_hidden.secrets`,
		`CREATE FUNCTION test_hidden.multiply(a int, b int) RETURNS int LANGUAGE sql AS 'SELECT a * b'`,
		// NO GRANT — test_restricted_user cannot see test_hidden
	}
	for _, stmt := range setup {
		if _, err := adminPool.Exec(ctx, stmt); err != nil {
			t.Fatalf("setup SQL %q: %v", stmt, err)
		}
	}

	t.Cleanup(func() {
		cleanup := []string{
			`DROP SCHEMA IF EXISTS test_visible CASCADE`,
			`DROP SCHEMA IF EXISTS test_hidden CASCADE`,
			`DROP ROLE IF EXISTS test_restricted_user`,
		}
		for _, stmt := range cleanup {
			adminPool.Exec(context.Background(), stmt)
		}
	})

	// Connect as restricted user
	restrictedURL := fmt.Sprintf("postgres://test_restricted_user:restricted_pass@localhost:%s/testdb?sslmode=disable&default_transaction_read_only=on", port)
	pool, err := pgxpool.New(ctx, restrictedURL)
	if err != nil {
		t.Fatalf("connecting as restricted user: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	return pool
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// TestDiscover_PartialSchemaAccess verifies that discovery succeeds when the
// connected user can only see a subset of schemas. This is the scenario that
// caused FK constraint failures before the information_schema JOIN fix.
func TestDiscover_PartialSchemaAccess(t *testing.T) {
	skipUnlessIntegration(t)

	pool := setupTestDB(t)
	store, err := knowledgemap.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	dbCfg := config.DatabaseConfig{
		Name:     "testdb",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		User:     "test_restricted_user",
	}

	err = Discover(context.Background(), pool, dbCfg, store)
	if err != nil {
		t.Fatalf("Discover failed (this was the FK constraint bug): %v", err)
	}

	t.Run("schemas", func(t *testing.T) { verifyDiscoveredSchemas(t, store) })
	t.Run("tables", func(t *testing.T) { verifyDiscoveredTables(t, store) })
	t.Run("describe", func(t *testing.T) { verifyDescribeTable(t, store) })
	t.Run("views", func(t *testing.T) { verifyDiscoveredViews(t, store) })
	t.Run("functions", func(t *testing.T) { verifyDiscoveredFunctions(t, store) })
}

func verifyDiscoveredSchemas(t *testing.T, store *knowledgemap.Store) {
	t.Helper()
	schemas, err := store.ListSchemas("testdb")
	if err != nil {
		t.Fatalf("ListSchemas: %v", err)
	}
	var foundVisible, foundHidden bool
	for _, s := range schemas {
		if s.SchemaName == "test_visible" {
			foundVisible = true
		}
		if s.SchemaName == "test_hidden" {
			foundHidden = true
		}
	}
	if !foundVisible {
		t.Error("expected test_visible schema to be discovered")
	}
	if foundHidden {
		t.Error("test_hidden schema should NOT be discovered (user has no access)")
	}
}

func verifyDiscoveredTables(t *testing.T, store *knowledgemap.Store) {
	t.Helper()
	tables, err := store.ListTables("testdb", "test_visible")
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	tableNames := make(map[string]bool)
	for _, tbl := range tables {
		tableNames[tbl.TableName] = true
	}
	if !tableNames["users"] || !tableNames["orders"] {
		t.Errorf("expected users and orders tables, got %v", tableNames)
	}

	hiddenTables, err := store.ListTables("testdb", "test_hidden")
	if err != nil {
		t.Fatalf("ListTables(hidden): %v", err)
	}
	if len(hiddenTables) > 0 {
		t.Errorf("expected no tables from hidden schema, got %d", len(hiddenTables))
	}
}

func verifyDescribeTable(t *testing.T, store *knowledgemap.Store) {
	t.Helper()
	detail, err := store.DescribeTable("testdb", "test_visible", "orders")
	if err != nil {
		t.Fatalf("DescribeTable: %v", err)
	}
	if len(detail.Columns) == 0 {
		t.Error("expected columns for orders table")
	}
	if len(detail.Constraints) == 0 {
		t.Error("expected constraints for orders table (PK + FK)")
	}
	if len(detail.Indexes) == 0 {
		t.Error("expected indexes for orders table")
	}
	if len(detail.ForeignKeys) == 0 {
		t.Error("expected foreign keys for orders table")
	}
}

func verifyDiscoveredViews(t *testing.T, store *knowledgemap.Store) {
	t.Helper()
	views, err := store.ListViews("testdb", "test_visible")
	if err != nil {
		t.Fatalf("ListViews(visible): %v", err)
	}
	if len(views) == 0 {
		t.Error("expected views in test_visible schema")
	}

	hiddenViews, err := store.ListViews("testdb", "test_hidden")
	if err != nil {
		t.Fatalf("ListViews(hidden): %v", err)
	}
	if len(hiddenViews) > 0 {
		t.Errorf("expected no views from hidden schema, got %d", len(hiddenViews))
	}
}

func verifyDiscoveredFunctions(t *testing.T, store *knowledgemap.Store) {
	t.Helper()
	funcs, err := store.ListFunctions("testdb", "test_visible")
	if err != nil {
		t.Fatalf("ListFunctions(visible): %v", err)
	}
	if len(funcs) == 0 {
		t.Error("expected functions in test_visible schema")
	}

	hiddenFuncs, err := store.ListFunctions("testdb", "test_hidden")
	if err != nil {
		t.Fatalf("ListFunctions(hidden): %v", err)
	}
	if len(hiddenFuncs) > 0 {
		t.Errorf("expected no functions from hidden schema, got %d", len(hiddenFuncs))
	}
}

// TestDiscover_WithSchemaFilter verifies that discovery respects schema filters
// even when the user has access to multiple schemas.
func TestDiscover_WithSchemaFilter(t *testing.T) {
	skipUnlessIntegration(t)

	pool := setupTestDB(t)
	store, err := knowledgemap.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	dbCfg := config.DatabaseConfig{
		Name:     "testdb",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		User:     "test_restricted_user",
		Schemas:  []string{"test_visible"}, // Only discover this schema
	}

	if err := Discover(context.Background(), pool, dbCfg, store); err != nil {
		t.Fatalf("Discover: %v", err)
	}

	schemas, err := store.ListSchemas("testdb")
	if err != nil {
		t.Fatalf("ListSchemas: %v", err)
	}
	if len(schemas) != 1 || schemas[0].SchemaName != "test_visible" {
		t.Errorf("expected only test_visible schema, got %v", schemas)
	}
}

// TestDiscover_Rediscovery verifies that running discovery twice replaces
// data cleanly without FK violations from stale data.
func TestDiscover_Rediscovery(t *testing.T) {
	skipUnlessIntegration(t)

	pool := setupTestDB(t)
	store, err := knowledgemap.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	dbCfg := config.DatabaseConfig{
		Name:     "testdb",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		User:     "test_restricted_user",
	}

	// First discovery
	if err := Discover(context.Background(), pool, dbCfg, store); err != nil {
		t.Fatalf("first Discover: %v", err)
	}

	// Second discovery should succeed (clear + re-insert)
	if err := Discover(context.Background(), pool, dbCfg, store); err != nil {
		t.Fatalf("second Discover: %v", err)
	}

	// Verify data is consistent
	tables, err := store.ListTables("testdb", "test_visible")
	if err != nil {
		t.Fatalf("ListTables: %v", err)
	}
	if len(tables) < 2 {
		t.Errorf("expected at least 2 tables after rediscovery, got %d", len(tables))
	}
}
