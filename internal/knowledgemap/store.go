package knowledgemap

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// Store provides access to the SQLite knowledge map database.
type Store struct {
	db *sqlx.DB
}

// Open creates or opens a SQLite database at the given path and initializes
// the schema. WAL mode and foreign keys are enabled.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("creating knowledge map directory %q: %w", dir, err)
		}
	}
	db, err := sqlx.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}
	// Allow concurrent readers under WAL mode while keeping the pool small.
	// WAL mode handles reader/writer concurrency; busy_timeout handles
	// write contention from concurrent discovery goroutines.
	db.SetMaxOpenConns(4)
	if _, err := db.Exec(ddl); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// ClearDatabase removes all knowledge map data for a given database name.
// Uses CASCADE delete from km_databases plus explicit FTS cleanup.
func (s *Store) ClearDatabase(dbName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear FTS index for this database (not covered by CASCADE)
	if _, err := tx.Exec("DELETE FROM km_search WHERE database_name = ?", dbName); err != nil {
		return fmt.Errorf("clearing km_search: %w", err)
	}

	// CASCADE handles all child tables
	if _, err := tx.Exec("DELETE FROM km_databases WHERE name = ?", dbName); err != nil {
		return fmt.Errorf("clearing km_databases: %w", err)
	}

	return tx.Commit()
}

func (s *Store) InsertDatabase(name, host string, port int, database string) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO km_databases (name, host, port, database, discovered_at) VALUES (?, ?, ?, ?, ?)",
		name, host, port, database, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) InsertSchema(dbName, schemaName string) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO km_schemas (database_name, schema_name) VALUES (?, ?)",
		dbName, schemaName,
	)
	return err
}

type TableInfo struct {
	SchemaName  string
	TableName   string
	TableType   string
	RowEstimate int64
	SizeBytes   int64
	Description string
}

func (s *Store) InsertTable(dbName string, t TableInfo) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO km_tables
		(database_name, schema_name, table_name, table_type, row_estimate, size_bytes, description)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		dbName, t.SchemaName, t.TableName, t.TableType, t.RowEstimate, t.SizeBytes, t.Description,
	)
	return err
}

type ColumnInfo struct {
	SchemaName    string
	TableName     string
	ColumnName    string
	Ordinal       int
	DataType      string
	IsNullable    bool
	ColumnDefault *string
	Description   string
}

func (s *Store) InsertColumn(dbName string, c ColumnInfo) error {
	nullable := 0
	if c.IsNullable {
		nullable = 1
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO km_columns
		(database_name, schema_name, table_name, column_name, ordinal, data_type, is_nullable, column_default, description)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dbName, c.SchemaName, c.TableName, c.ColumnName, c.Ordinal, c.DataType, nullable, c.ColumnDefault, c.Description,
	)
	return err
}

type ConstraintInfo struct {
	SchemaName     string
	TableName      string
	ConstraintName string
	ConstraintType string
	Definition     string
}

func (s *Store) InsertConstraint(dbName string, c ConstraintInfo) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO km_constraints
		(database_name, schema_name, table_name, constraint_name, constraint_type, definition)
		VALUES (?, ?, ?, ?, ?, ?)`,
		dbName, c.SchemaName, c.TableName, c.ConstraintName, c.ConstraintType, c.Definition,
	)
	return err
}

type IndexInfo struct {
	SchemaName string
	TableName  string
	IndexName  string
	IsUnique   bool
	IsPrimary  bool
	Definition string
}

func (s *Store) InsertIndex(dbName string, idx IndexInfo) error {
	unique, primary := 0, 0
	if idx.IsUnique {
		unique = 1
	}
	if idx.IsPrimary {
		primary = 1
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO km_indexes
		(database_name, schema_name, table_name, index_name, is_unique, is_primary, definition)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		dbName, idx.SchemaName, idx.TableName, idx.IndexName, unique, primary, idx.Definition,
	)
	return err
}

type ForeignKeyInfo struct {
	SchemaName     string
	TableName      string
	ConstraintName string
	ColumnName     string
	RefSchema      string
	RefTable       string
	RefColumn      string
}

func (s *Store) InsertForeignKey(dbName string, fk ForeignKeyInfo) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO km_foreign_keys
		(database_name, schema_name, table_name, constraint_name, column_name, ref_schema, ref_table, ref_column)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		dbName, fk.SchemaName, fk.TableName, fk.ConstraintName, fk.ColumnName, fk.RefSchema, fk.RefTable, fk.RefColumn,
	)
	return err
}

type ViewInfo struct {
	SchemaName  string
	ViewName    string
	Definition  string
	Description string
}

func (s *Store) InsertView(dbName string, v ViewInfo) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO km_views
		(database_name, schema_name, view_name, definition, description)
		VALUES (?, ?, ?, ?, ?)`,
		dbName, v.SchemaName, v.ViewName, v.Definition, v.Description,
	)
	return err
}

type FunctionInfo struct {
	SchemaName   string
	FunctionName string
	ResultType   string
	ArgTypes     string
	Description  string
	Language     string
}

func (s *Store) InsertFunction(dbName string, f FunctionInfo) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO km_functions
		(database_name, schema_name, function_name, result_type, argument_types, description, language)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		dbName, f.SchemaName, f.FunctionName, f.ResultType, f.ArgTypes, f.Description, f.Language,
	)
	return err
}

// IndexForSearch populates the FTS5 search index for a database.
// Runs atomically in a transaction to prevent partial index corruption.
func (s *Store) IndexForSearch(dbName string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM km_search WHERE database_name = ?", dbName); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO km_search (database_name, schema_name, object_type, object_name, detail)
		SELECT database_name, schema_name, 'table', table_name, COALESCE(description, '')
		FROM km_tables WHERE database_name = ?`, dbName); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO km_search (database_name, schema_name, object_type, object_name, detail)
		SELECT database_name, schema_name, 'column', table_name || '.' || column_name, data_type || ' ' || COALESCE(description, '')
		FROM km_columns WHERE database_name = ?`, dbName); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO km_search (database_name, schema_name, object_type, object_name, detail)
		SELECT database_name, schema_name, 'view', view_name, COALESCE(description, '')
		FROM km_views WHERE database_name = ?`, dbName); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO km_search (database_name, schema_name, object_type, object_name, detail)
		SELECT database_name, schema_name, 'function', function_name, COALESCE(result_type, '') || ' ' || COALESCE(argument_types, '') || ' ' || COALESCE(description, '')
		FROM km_functions WHERE database_name = ?`, dbName); err != nil {
		return err
	}

	return tx.Commit()
}
