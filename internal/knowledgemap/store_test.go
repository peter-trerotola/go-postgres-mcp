package knowledgemap

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func seedTestData(t *testing.T, store *Store) {
	t.Helper()
	if err := store.InsertDatabase("testdb", "localhost", 5432, "mydb"); err != nil {
		t.Fatalf("insert database: %v", err)
	}
	if err := store.InsertSchema("testdb", "public"); err != nil {
		t.Fatalf("insert schema: %v", err)
	}
	if err := store.InsertTable("testdb", TableInfo{
		SchemaName: "public", TableName: "users", TableType: "BASE TABLE",
		RowEstimate: 1000, SizeBytes: 65536, Description: "User accounts",
	}); err != nil {
		t.Fatalf("insert table: %v", err)
	}
	if err := store.InsertTable("testdb", TableInfo{
		SchemaName: "public", TableName: "orders", TableType: "BASE TABLE",
		RowEstimate: 5000, SizeBytes: 131072, Description: "Customer orders",
	}); err != nil {
		t.Fatalf("insert table: %v", err)
	}
	defVal := "nextval('users_id_seq')"
	if err := store.InsertColumn("testdb", ColumnInfo{
		SchemaName: "public", TableName: "users", ColumnName: "id",
		Ordinal: 1, DataType: "integer", IsNullable: false,
		ColumnDefault: &defVal, Description: "Primary key",
	}); err != nil {
		t.Fatalf("insert column: %v", err)
	}
	if err := store.InsertColumn("testdb", ColumnInfo{
		SchemaName: "public", TableName: "users", ColumnName: "name",
		Ordinal: 2, DataType: "text", IsNullable: false,
		Description: "User name",
	}); err != nil {
		t.Fatalf("insert column: %v", err)
	}
	if err := store.InsertColumn("testdb", ColumnInfo{
		SchemaName: "public", TableName: "users", ColumnName: "email",
		Ordinal: 3, DataType: "text", IsNullable: true,
		Description: "Email address",
	}); err != nil {
		t.Fatalf("insert column: %v", err)
	}
	if err := store.InsertConstraint("testdb", ConstraintInfo{
		SchemaName: "public", TableName: "users", ConstraintName: "users_pkey",
		ConstraintType: "PRIMARY KEY", Definition: "PRIMARY KEY (id)",
	}); err != nil {
		t.Fatalf("insert constraint: %v", err)
	}
	if err := store.InsertIndex("testdb", IndexInfo{
		SchemaName: "public", TableName: "users", IndexName: "users_pkey",
		IsUnique: true, IsPrimary: true, Definition: "CREATE UNIQUE INDEX users_pkey ON public.users USING btree (id)",
	}); err != nil {
		t.Fatalf("insert index: %v", err)
	}
	if err := store.InsertIndex("testdb", IndexInfo{
		SchemaName: "public", TableName: "users", IndexName: "users_email_idx",
		IsUnique: true, IsPrimary: false, Definition: "CREATE UNIQUE INDEX users_email_idx ON public.users USING btree (email)",
	}); err != nil {
		t.Fatalf("insert index: %v", err)
	}
	if err := store.InsertColumn("testdb", ColumnInfo{
		SchemaName: "public", TableName: "orders", ColumnName: "user_id",
		Ordinal: 2, DataType: "integer", IsNullable: false,
	}); err != nil {
		t.Fatalf("insert column: %v", err)
	}
	if err := store.InsertForeignKey("testdb", ForeignKeyInfo{
		SchemaName: "public", TableName: "orders", ConstraintName: "orders_user_id_fkey",
		ColumnName: "user_id", RefSchema: "public", RefTable: "users", RefColumn: "id",
	}); err != nil {
		t.Fatalf("insert foreign key: %v", err)
	}
	if err := store.InsertView("testdb", ViewInfo{
		SchemaName: "public", ViewName: "active_users",
		Definition: "SELECT * FROM users WHERE active = true", Description: "Active users only",
	}); err != nil {
		t.Fatalf("insert view: %v", err)
	}
	if err := store.InsertFunction("testdb", FunctionInfo{
		SchemaName: "public", FunctionName: "get_user_count",
		ResultType: "integer", ArgTypes: "", Description: "Returns total user count",
		Language: "sql",
	}); err != nil {
		t.Fatalf("insert function: %v", err)
	}
}

func TestOpen_CreatesTables(t *testing.T) {
	store := openTestStore(t)
	// Verify tables exist by running queries against them
	_, err := store.ListDatabases()
	if err != nil {
		t.Errorf("ListDatabases failed: %v", err)
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/dir/test.db")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestInsertDatabase(t *testing.T) {
	store := openTestStore(t)
	if err := store.InsertDatabase("test", "localhost", 5432, "mydb"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	dbs, err := store.ListDatabases()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(dbs) != 1 {
		t.Fatalf("expected 1 database, got %d", len(dbs))
	}
	if dbs[0].Name != "test" {
		t.Errorf("expected name %q, got %q", "test", dbs[0].Name)
	}
}

func TestInsertDatabase_Replace(t *testing.T) {
	store := openTestStore(t)
	store.InsertDatabase("test", "host1", 5432, "db1")
	store.InsertDatabase("test", "host2", 5433, "db2")
	dbs, _ := store.ListDatabases()
	if len(dbs) != 1 {
		t.Fatalf("expected 1 database after replace, got %d", len(dbs))
	}
	if dbs[0].Host != "host2" {
		t.Errorf("expected updated host, got %q", dbs[0].Host)
	}
}

func TestListSchemas(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)
	schemas, err := store.ListSchemas("testdb")
	if err != nil {
		t.Fatalf("list schemas: %v", err)
	}
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	if schemas[0].SchemaName != "public" {
		t.Errorf("expected %q, got %q", "public", schemas[0].SchemaName)
	}
}

func TestListSchemas_UnknownDB(t *testing.T) {
	store := openTestStore(t)
	schemas, err := store.ListSchemas("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(schemas) != 0 {
		t.Errorf("expected 0 schemas, got %d", len(schemas))
	}
}

func TestListTables(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)
	tables, err := store.ListTables("testdb", "public")
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(tables))
	}
	// Ordered by table_name
	if tables[0].TableName != "orders" {
		t.Errorf("expected first table 'orders', got %q", tables[0].TableName)
	}
	if tables[1].TableName != "users" {
		t.Errorf("expected second table 'users', got %q", tables[1].TableName)
	}
}

func TestDescribeTable(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)

	detail, err := store.DescribeTable("testdb", "public", "users")
	if err != nil {
		t.Fatalf("describe table: %v", err)
	}

	// Table info
	if detail.Table.TableName != "users" {
		t.Errorf("expected table name 'users', got %q", detail.Table.TableName)
	}
	if detail.Table.RowEstimate != 1000 {
		t.Errorf("expected row estimate 1000, got %d", detail.Table.RowEstimate)
	}

	// Columns (ordered by ordinal)
	if len(detail.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(detail.Columns))
	}
	if detail.Columns[0].ColumnName != "id" {
		t.Errorf("expected first column 'id', got %q", detail.Columns[0].ColumnName)
	}
	if detail.Columns[0].IsNullable {
		t.Error("expected id to be NOT NULL")
	}
	if detail.Columns[2].ColumnName != "email" {
		t.Errorf("expected third column 'email', got %q", detail.Columns[2].ColumnName)
	}
	if !detail.Columns[2].IsNullable {
		t.Error("expected email to be nullable")
	}

	// Constraints
	if len(detail.Constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(detail.Constraints))
	}
	if detail.Constraints[0].ConstraintType != "PRIMARY KEY" {
		t.Errorf("expected PRIMARY KEY, got %q", detail.Constraints[0].ConstraintType)
	}

	// Indexes
	if len(detail.Indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(detail.Indexes))
	}
}

func TestDescribeTable_NotFound(t *testing.T) {
	store := openTestStore(t)
	_, err := store.DescribeTable("testdb", "public", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent table")
	}
}

func TestListViews(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)
	views, err := store.ListViews("testdb", "public")
	if err != nil {
		t.Fatalf("list views: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}
	if views[0].ViewName != "active_users" {
		t.Errorf("expected view 'active_users', got %q", views[0].ViewName)
	}
}

func TestListFunctions(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)
	fns, err := store.ListFunctions("testdb", "public")
	if err != nil {
		t.Fatalf("list functions: %v", err)
	}
	if len(fns) != 1 {
		t.Fatalf("expected 1 function, got %d", len(fns))
	}
	if fns[0].FunctionName != "get_user_count" {
		t.Errorf("expected function 'get_user_count', got %q", fns[0].FunctionName)
	}
}

func TestClearDatabase(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)

	// Verify data exists
	dbs, _ := store.ListDatabases()
	if len(dbs) != 1 {
		t.Fatalf("expected 1 database before clear, got %d", len(dbs))
	}

	if err := store.ClearDatabase("testdb"); err != nil {
		t.Fatalf("clear database: %v", err)
	}

	// Verify all data is gone
	dbs, _ = store.ListDatabases()
	if len(dbs) != 0 {
		t.Errorf("expected 0 databases after clear, got %d", len(dbs))
	}
	schemas, _ := store.ListSchemas("testdb")
	if len(schemas) != 0 {
		t.Errorf("expected 0 schemas after clear, got %d", len(schemas))
	}
	tables, _ := store.ListTables("testdb", "public")
	if len(tables) != 0 {
		t.Errorf("expected 0 tables after clear, got %d", len(tables))
	}
}

func TestClearDatabase_Nonexistent(t *testing.T) {
	store := openTestStore(t)
	// Should not error when clearing a nonexistent database
	if err := store.ClearDatabase("nonexistent"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIndexForSearch(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)

	if err := store.IndexForSearch("testdb"); err != nil {
		t.Fatalf("index for search: %v", err)
	}

	// Search for a table
	results, err := store.SearchSchema("users")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for 'users'")
	}

	// Verify we find the table itself
	found := false
	for _, r := range results {
		if r.ObjectType == "table" && r.ObjectName == "users" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find table 'users' in search results")
	}
}

func TestSearchSchema_NoResults(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)
	store.IndexForSearch("testdb")

	results, err := store.SearchSchema("xyznonexistent")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchSchema_FindsColumns(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)
	store.IndexForSearch("testdb")

	results, err := store.SearchSchema("email")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for 'email'")
	}
	found := false
	for _, r := range results {
		if r.ObjectType == "column" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find a column in search results")
	}
}

func TestSearchSchema_FindsViews(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)
	store.IndexForSearch("testdb")

	results, err := store.SearchSchema("active_users")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, r := range results {
		if r.ObjectType == "view" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find view in search results")
	}
}

func TestSearchSchema_FindsFunctions(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)
	store.IndexForSearch("testdb")

	results, err := store.SearchSchema("get_user_count")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, r := range results {
		if r.ObjectType == "function" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find function in search results")
	}
}

func TestSearchSchema_EmptyQuery(t *testing.T) {
	store := openTestStore(t)
	_, err := store.SearchSchema("")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSearchSchema_WhitespaceOnlyQuery(t *testing.T) {
	store := openTestStore(t)
	_, err := store.SearchSchema("   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only query")
	}
}

func TestIndexForSearch_ReindexClears(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)
	store.IndexForSearch("testdb")

	// Index again (should clear and re-index, not duplicate)
	store.IndexForSearch("testdb")

	results, _ := store.SearchSchema("users")
	// Count how many "table" results for "users" - should be exactly 1
	tableCount := 0
	for _, r := range results {
		if r.ObjectType == "table" && r.ObjectName == "users" {
			tableCount++
		}
	}
	if tableCount != 1 {
		t.Errorf("expected exactly 1 table result for 'users', got %d", tableCount)
	}
}

func TestForeignKeys_InDescribeTable(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)

	detail, err := store.DescribeTable("testdb", "public", "orders")
	if err != nil {
		t.Fatalf("describe table: %v", err)
	}
	if len(detail.ForeignKeys) != 1 {
		t.Fatalf("expected 1 foreign key, got %d", len(detail.ForeignKeys))
	}
	fk := detail.ForeignKeys[0]
	if fk.RefTable != "users" {
		t.Errorf("expected FK to 'users', got %q", fk.RefTable)
	}
	if fk.RefColumn != "id" {
		t.Errorf("expected FK to column 'id', got %q", fk.RefColumn)
	}
}

func TestMultipleSchemas(t *testing.T) {
	store := openTestStore(t)
	store.InsertDatabase("testdb", "localhost", 5432, "mydb")
	store.InsertSchema("testdb", "public")
	store.InsertSchema("testdb", "audit")

	schemas, _ := store.ListSchemas("testdb")
	if len(schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(schemas))
	}
	// Ordered alphabetically
	if schemas[0].SchemaName != "audit" {
		t.Errorf("expected first schema 'audit', got %q", schemas[0].SchemaName)
	}
}

func TestMultipleDatabases(t *testing.T) {
	store := openTestStore(t)
	store.InsertDatabase("db1", "h1", 5432, "d1")
	store.InsertDatabase("db2", "h2", 5432, "d2")

	dbs, _ := store.ListDatabases()
	if len(dbs) != 2 {
		t.Fatalf("expected 2 databases, got %d", len(dbs))
	}
}

func TestClearDatabase_OnlyAffectsTargetDB(t *testing.T) {
	store := openTestStore(t)
	store.InsertDatabase("db1", "h1", 5432, "d1")
	store.InsertDatabase("db2", "h2", 5432, "d2")
	store.InsertSchema("db1", "public")
	store.InsertSchema("db2", "public")

	store.ClearDatabase("db1")

	dbs, _ := store.ListDatabases()
	if len(dbs) != 1 {
		t.Fatalf("expected 1 database remaining, got %d", len(dbs))
	}
	if dbs[0].Name != "db2" {
		t.Errorf("expected db2 to remain, got %q", dbs[0].Name)
	}
}

func TestListColumnsCompact_ReturnsOrdinalOrder(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)

	cols, err := store.ListColumnsCompact("testdb", "public", "users")
	if err != nil {
		t.Fatalf("ListColumnsCompact: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	// Verify ordinal order
	expected := []ColumnSummary{
		{Column: "id", Type: "integer"},
		{Column: "name", Type: "text"},
		{Column: "email", Type: "text"},
	}
	for i, want := range expected {
		if cols[i].Column != want.Column {
			t.Errorf("col[%d].Column: expected %q, got %q", i, want.Column, cols[i].Column)
		}
		if cols[i].Type != want.Type {
			t.Errorf("col[%d].Type: expected %q, got %q", i, want.Type, cols[i].Type)
		}
	}
}

func TestListColumnsCompact_NonexistentTable(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)

	cols, err := store.ListColumnsCompact("testdb", "public", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cols) != 0 {
		t.Errorf("expected 0 columns for nonexistent table, got %d", len(cols))
	}
}

func TestListColumnsCompact_NonexistentDB(t *testing.T) {
	store := openTestStore(t)

	cols, err := store.ListColumnsCompact("nodb", "public", "users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cols) != 0 {
		t.Errorf("expected 0 columns, got %d", len(cols))
	}
}

func TestCountTables_Empty(t *testing.T) {
	store := openTestStore(t)

	count, err := store.CountTables()
	if err != nil {
		t.Fatalf("CountTables: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 tables, got %d", count)
	}
}

func TestCountTables_WithData(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)

	count, err := store.CountTables()
	if err != nil {
		t.Fatalf("CountTables: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 tables, got %d", count)
	}
}

func TestCountTables_MultipleDBs(t *testing.T) {
	store := openTestStore(t)
	seedTestData(t, store)

	// Add another database with one table
	store.InsertDatabase("db2", "h2", 5432, "d2")
	store.InsertSchema("db2", "public")
	store.InsertTable("db2", TableInfo{
		SchemaName: "public", TableName: "products", TableType: "BASE TABLE",
	})

	count, err := store.CountTables()
	if err != nil {
		t.Fatalf("CountTables: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 tables across 2 databases, got %d", count)
	}
}
