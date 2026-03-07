package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	yaml := `
databases:
  - name: "testdb"
    host: "localhost"
    port: 5432
    database: "mydb"
    user: "myuser"
    password_env: "TEST_DB_PASS"
    sslmode: "disable"

knowledgemap:
  path: "/tmp/km.db"
  auto_discover_on_startup: true
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Databases) != 1 {
		t.Fatalf("expected 1 database, got %d", len(cfg.Databases))
	}
	db := cfg.Databases[0]
	if db.Name != "testdb" {
		t.Errorf("expected name %q, got %q", "testdb", db.Name)
	}
	if db.Host != "localhost" {
		t.Errorf("expected host %q, got %q", "localhost", db.Host)
	}
	if db.Port != 5432 {
		t.Errorf("expected port %d, got %d", 5432, db.Port)
	}
	if db.Database != "mydb" {
		t.Errorf("expected database %q, got %q", "mydb", db.Database)
	}
	if db.User != "myuser" {
		t.Errorf("expected user %q, got %q", "myuser", db.User)
	}
	if db.PasswordEnv != "TEST_DB_PASS" {
		t.Errorf("expected password_env %q, got %q", "TEST_DB_PASS", db.PasswordEnv)
	}
	if db.SSLMode != "disable" {
		t.Errorf("expected sslmode %q, got %q", "disable", db.SSLMode)
	}
	if cfg.KnowledgeMap.Path != "/tmp/km.db" {
		t.Errorf("expected km path %q, got %q", "/tmp/km.db", cfg.KnowledgeMap.Path)
	}
	if !cfg.KnowledgeMap.AutoDiscoverOnStartup {
		t.Error("expected auto_discover_on_startup to be true")
	}
}

func TestLoad_MultipleDatabases(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
  - name: "db2"
    host: "h2"
    database: "d2"
    user: "u2"
    password_env: "P2"
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Databases) != 2 {
		t.Fatalf("expected 2 databases, got %d", len(cfg.Databases))
	}
}

func TestLoad_DefaultKnowledgeMapPath(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.KnowledgeMap.Path != "./knowledgemap.db" {
		t.Errorf("expected default km path, got %q", cfg.KnowledgeMap.Path)
	}
}

func TestLoad_DefaultPort(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Setenv("P1", "secret")
	connStr, err := cfg.Databases[0].ConnString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(connStr, ":5432/") {
		t.Errorf("expected default port 5432 in conn string, got %s", connStr)
	}
}

func TestLoad_DefaultSSLMode(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Setenv("P1", "secret")
	connStr, err := cfg.Databases[0].ConnString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(connStr, "sslmode=prefer") {
		t.Errorf("expected default sslmode=prefer, got %s", connStr)
	}
}

func TestValidation_NoDatabases(t *testing.T) {
	yaml := `databases: []`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty databases")
	}
	if !strings.Contains(err.Error(), "at least one database") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_MissingName(t *testing.T) {
	yaml := `
databases:
  - host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_DuplicateName(t *testing.T) {
	yaml := `
databases:
  - name: "dup"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
  - name: "dup"
    host: "h2"
    database: "d2"
    user: "u2"
    password_env: "P2"
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for duplicate names")
	}
	if !strings.Contains(err.Error(), "duplicate database name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_MissingHost(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    database: "d1"
    user: "u1"
    password_env: "P1"
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing host")
	}
	if !strings.Contains(err.Error(), "host is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_MissingDatabase(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    user: "u1"
    password_env: "P1"
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing database")
	}
	if !strings.Contains(err.Error(), "database is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_MissingUser(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    password_env: "P1"
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing user")
	}
	if !strings.Contains(err.Error(), "user is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_MissingPasswordEnv(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing password_env")
	}
	if !strings.Contains(err.Error(), "password_env is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTemp(t, "not: [valid: yaml:")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	_, err := Load("/tmp/nonexistent_config_file_12345.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestPassword_Set(t *testing.T) {
	t.Setenv("TEST_PASS_XYZ", "mysecret")
	db := DatabaseConfig{Name: "test", PasswordEnv: "TEST_PASS_XYZ"}
	pw, err := db.Password()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pw != "mysecret" {
		t.Errorf("expected %q, got %q", "mysecret", pw)
	}
}

func TestPassword_Unset(t *testing.T) {
	db := DatabaseConfig{Name: "test", PasswordEnv: "UNSET_ENV_VAR_XYZ_12345"}
	os.Unsetenv("UNSET_ENV_VAR_XYZ_12345")
	_, err := db.Password()
	if err == nil {
		t.Fatal("expected error for unset env var")
	}
	if !strings.Contains(err.Error(), "not set or empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConnString_ReadOnlyParam(t *testing.T) {
	t.Setenv("TEST_CONN_PASS", "secret")
	db := DatabaseConfig{
		Name:        "test",
		Host:        "localhost",
		Port:        5432,
		Database:    "mydb",
		User:        "myuser",
		PasswordEnv: "TEST_CONN_PASS",
		SSLMode:     "disable",
	}
	connStr, err := db.ConnString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(connStr, "default_transaction_read_only=on") {
		t.Errorf("connection string missing read-only param: %s", connStr)
	}
}

func TestConnString_SpecialCharactersInPassword(t *testing.T) {
	t.Setenv("TEST_SPECIAL_PASS", "p@ss:w/rd#?&+%")
	db := DatabaseConfig{
		Name:        "test",
		Host:        "localhost",
		Port:        5432,
		Database:    "mydb",
		User:        "myuser",
		PasswordEnv: "TEST_SPECIAL_PASS",
		SSLMode:     "disable",
	}
	connStr, err := db.ConnString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not contain raw @ or # in the password position
	if strings.Contains(connStr, "p@ss") {
		t.Errorf("password not URL-encoded: %s", connStr)
	}
	// Should contain the URL-encoded form
	if !strings.Contains(connStr, "p%40ss") {
		t.Errorf("expected URL-encoded password in conn string: %s", connStr)
	}
}

func TestConnString_SpecialCharactersInUser(t *testing.T) {
	t.Setenv("TEST_USER_PASS", "secret")
	db := DatabaseConfig{
		Name:        "test",
		Host:        "localhost",
		Port:        5432,
		Database:    "mydb",
		User:        "user@domain",
		PasswordEnv: "TEST_USER_PASS",
		SSLMode:     "disable",
	}
	connStr, err := db.ConnString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(connStr, "user@domain") {
		t.Errorf("user not URL-encoded: %s", connStr)
	}
}

func TestConnString_InvalidSSLMode(t *testing.T) {
	t.Setenv("TEST_SSL_PASS", "secret")
	db := DatabaseConfig{
		Name:        "test",
		Host:        "localhost",
		Port:        5432,
		Database:    "mydb",
		User:        "myuser",
		PasswordEnv: "TEST_SSL_PASS",
		SSLMode:     "disable&options=-c statement_timeout=0",
	}
	_, err := db.ConnString()
	if err == nil {
		t.Fatal("expected error for invalid sslmode")
	}
	if !strings.Contains(err.Error(), "invalid sslmode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConnString_MissingPassword(t *testing.T) {
	os.Unsetenv("MISSING_PASS_XYZ")
	db := DatabaseConfig{
		Name:        "test",
		Host:        "localhost",
		Database:    "mydb",
		User:        "myuser",
		PasswordEnv: "MISSING_PASS_XYZ",
	}
	_, err := db.ConnString()
	if err == nil {
		t.Fatal("expected error for missing password")
	}
}

// --- Schema Filter Tests ---

func TestLoad_WithSchemas(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
    schemas:
      - "public"
      - "billing"
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Databases[0].Schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(cfg.Databases[0].Schemas))
	}
	if cfg.Databases[0].Schemas[0] != "public" {
		t.Errorf("expected first schema 'public', got %q", cfg.Databases[0].Schemas[0])
	}
}

func TestValidation_EmptySchemaEntry(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
    schemas:
      - "public"
      - ""
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty schema entry")
	}
	if !strings.Contains(err.Error(), "schemas[1] is empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestShouldIncludeSchema_NoFilter(t *testing.T) {
	db := DatabaseConfig{Name: "test"}
	if !db.ShouldIncludeSchema("anything") {
		t.Error("expected all schemas included when no filter set")
	}
}

func TestShouldIncludeSchema_WithFilter(t *testing.T) {
	db := DatabaseConfig{Name: "test", Schemas: []string{"public", "billing"}}
	if !db.ShouldIncludeSchema("public") {
		t.Error("expected 'public' to be included")
	}
	if !db.ShouldIncludeSchema("billing") {
		t.Error("expected 'billing' to be included")
	}
	if db.ShouldIncludeSchema("audit") {
		t.Error("expected 'audit' to be excluded")
	}
}

// --- Table Filter Tests ---

func TestLoad_WithTableInclude(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
    tables:
      include:
        - "public.users"
        - "public.orders"
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Databases[0].Tables.Include) != 2 {
		t.Fatalf("expected 2 includes, got %d", len(cfg.Databases[0].Tables.Include))
	}
}

func TestLoad_WithTableExclude(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
    tables:
      exclude:
        - "public.migrations"
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Databases[0].Tables.Exclude) != 1 {
		t.Fatalf("expected 1 exclude, got %d", len(cfg.Databases[0].Tables.Exclude))
	}
}

func TestValidation_IncludeAndExcludeMutuallyExclusive(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
    tables:
      include:
        - "public.users"
      exclude:
        - "public.migrations"
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for include+exclude")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_TableIncludeMustBeQualified(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
    tables:
      include:
        - "users"
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unqualified table name")
	}
	if !strings.Contains(err.Error(), "schema.table format") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidation_TableExcludeMustBeQualified(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
    tables:
      exclude:
        - "migrations"
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unqualified table name")
	}
	if !strings.Contains(err.Error(), "schema.table format") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestShouldIncludeTable_NoFilter(t *testing.T) {
	db := DatabaseConfig{Name: "test"}
	if !db.ShouldIncludeTable("public", "users") {
		t.Error("expected all tables included when no filter set")
	}
}

func TestShouldIncludeTable_Include(t *testing.T) {
	db := DatabaseConfig{
		Name:   "test",
		Tables: TableFilter{Include: []string{"public.users", "public.orders"}},
	}
	if !db.ShouldIncludeTable("public", "users") {
		t.Error("expected 'public.users' to be included")
	}
	if !db.ShouldIncludeTable("public", "orders") {
		t.Error("expected 'public.orders' to be included")
	}
	if db.ShouldIncludeTable("public", "migrations") {
		t.Error("expected 'public.migrations' to be excluded")
	}
	if db.ShouldIncludeTable("billing", "invoices") {
		t.Error("expected 'billing.invoices' to be excluded")
	}
}

func TestShouldIncludeTable_Exclude(t *testing.T) {
	db := DatabaseConfig{
		Name:   "test",
		Tables: TableFilter{Exclude: []string{"public.migrations", "public.sessions"}},
	}
	if !db.ShouldIncludeTable("public", "users") {
		t.Error("expected 'public.users' to be included")
	}
	if db.ShouldIncludeTable("public", "migrations") {
		t.Error("expected 'public.migrations' to be excluded")
	}
	if db.ShouldIncludeTable("public", "sessions") {
		t.Error("expected 'public.sessions' to be excluded")
	}
	if !db.ShouldIncludeTable("billing", "invoices") {
		t.Error("expected 'billing.invoices' to be included")
	}
}

// --- Combined Schema + Table Filter Tests ---

func TestLoad_SchemasAndTableFilter(t *testing.T) {
	yaml := `
databases:
  - name: "db1"
    host: "h1"
    database: "d1"
    user: "u1"
    password_env: "P1"
    schemas:
      - "public"
    tables:
      exclude:
        - "public.migrations"
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	db := cfg.Databases[0]
	if !db.ShouldIncludeSchema("public") {
		t.Error("public should be included")
	}
	if db.ShouldIncludeSchema("billing") {
		t.Error("billing should be excluded by schema filter")
	}
	if !db.ShouldIncludeTable("public", "users") {
		t.Error("public.users should be included")
	}
	if db.ShouldIncludeTable("public", "migrations") {
		t.Error("public.migrations should be excluded by table filter")
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
