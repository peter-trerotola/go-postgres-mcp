package knowledgemap

import (
	"database/sql"
	"fmt"
	"strings"
)

type DatabaseRow struct {
	Name         string `json:"name" db:"name"`
	Host         string `json:"host" db:"host"`
	Port         int    `json:"port" db:"port"`
	Database     string `json:"database" db:"database"`
	DiscoveredAt string `json:"discovered_at" db:"discovered_at"`
}

func (s *Store) ListDatabases() ([]DatabaseRow, error) {
	var result []DatabaseRow
	err := s.db.Select(&result, "SELECT name, host, port, database, discovered_at FROM km_databases ORDER BY name")
	return result, err
}

type SchemaRow struct {
	SchemaName string `json:"schema_name" db:"schema_name"`
}

func (s *Store) ListSchemas(dbName string) ([]SchemaRow, error) {
	var result []SchemaRow
	err := s.db.Select(&result,
		"SELECT schema_name FROM km_schemas WHERE database_name = ? ORDER BY schema_name",
		dbName,
	)
	return result, err
}

type TableRow struct {
	SchemaName  string         `json:"schema_name" db:"schema_name"`
	TableName   string         `json:"table_name" db:"table_name"`
	TableType   string         `json:"table_type" db:"table_type"`
	RowEstimate int64          `json:"row_estimate" db:"row_estimate"`
	SizeBytes   int64          `json:"size_bytes" db:"size_bytes"`
	Description sql.NullString `json:"description" db:"description"`
}

func (s *Store) ListTables(dbName, schemaName string) ([]TableRow, error) {
	var result []TableRow
	err := s.db.Select(&result,
		`SELECT schema_name, table_name, table_type, row_estimate, size_bytes, description
		FROM km_tables WHERE database_name = ? AND schema_name = ?
		ORDER BY table_name`,
		dbName, schemaName,
	)
	return result, err
}

type ColumnRow struct {
	ColumnName    string         `json:"column_name" db:"column_name"`
	Ordinal       int            `json:"ordinal" db:"ordinal"`
	DataType      string         `json:"data_type" db:"data_type"`
	IsNullable    bool           `json:"is_nullable" db:"is_nullable"`
	ColumnDefault sql.NullString `json:"column_default" db:"column_default"`
	Description   sql.NullString `json:"description" db:"description"`
}

type ConstraintRow struct {
	ConstraintName string         `json:"constraint_name" db:"constraint_name"`
	ConstraintType string         `json:"constraint_type" db:"constraint_type"`
	Definition     sql.NullString `json:"definition" db:"definition"`
}

type IndexRow struct {
	IndexName  string         `json:"index_name" db:"index_name"`
	IsUnique   bool           `json:"is_unique" db:"is_unique"`
	IsPrimary  bool           `json:"is_primary" db:"is_primary"`
	Definition sql.NullString `json:"definition" db:"definition"`
}

type ForeignKeyRow struct {
	ConstraintName string `json:"constraint_name" db:"constraint_name"`
	ColumnName     string `json:"column_name" db:"column_name"`
	RefSchema      string `json:"ref_schema" db:"ref_schema"`
	RefTable       string `json:"ref_table" db:"ref_table"`
	RefColumn      string `json:"ref_column" db:"ref_column"`
}

type TableDetail struct {
	Table       TableRow        `json:"table"`
	Columns     []ColumnRow     `json:"columns"`
	Constraints []ConstraintRow `json:"constraints"`
	Indexes     []IndexRow      `json:"indexes"`
	ForeignKeys []ForeignKeyRow `json:"foreign_keys"`
}

func (s *Store) DescribeTable(dbName, schemaName, tableName string) (*TableDetail, error) {
	var t TableRow
	err := s.db.Get(&t,
		`SELECT schema_name, table_name, table_type, row_estimate, size_bytes, description
		FROM km_tables WHERE database_name = ? AND schema_name = ? AND table_name = ?`,
		dbName, schemaName, tableName,
	)
	if err != nil {
		return nil, err
	}

	detail := &TableDetail{Table: t}

	if err := s.db.Select(&detail.Columns,
		`SELECT column_name, ordinal, data_type, is_nullable, column_default, description
		FROM km_columns WHERE database_name = ? AND schema_name = ? AND table_name = ?
		ORDER BY ordinal`,
		dbName, schemaName, tableName,
	); err != nil {
		return nil, err
	}

	if err := s.db.Select(&detail.Constraints,
		`SELECT constraint_name, constraint_type, definition
		FROM km_constraints WHERE database_name = ? AND schema_name = ? AND table_name = ?
		ORDER BY constraint_name`,
		dbName, schemaName, tableName,
	); err != nil {
		return nil, err
	}

	if err := s.db.Select(&detail.Indexes,
		`SELECT index_name, is_unique, is_primary, definition
		FROM km_indexes WHERE database_name = ? AND schema_name = ? AND table_name = ?
		ORDER BY index_name`,
		dbName, schemaName, tableName,
	); err != nil {
		return nil, err
	}

	if err := s.db.Select(&detail.ForeignKeys,
		`SELECT constraint_name, column_name, ref_schema, ref_table, ref_column
		FROM km_foreign_keys WHERE database_name = ? AND schema_name = ? AND table_name = ?
		ORDER BY constraint_name, column_name`,
		dbName, schemaName, tableName,
	); err != nil {
		return nil, err
	}

	return detail, nil
}

type ViewRow struct {
	SchemaName  string         `json:"schema_name" db:"schema_name"`
	ViewName    string         `json:"view_name" db:"view_name"`
	Definition  sql.NullString `json:"definition" db:"definition"`
	Description sql.NullString `json:"description" db:"description"`
}

func (s *Store) ListViews(dbName, schemaName string) ([]ViewRow, error) {
	var result []ViewRow
	err := s.db.Select(&result,
		`SELECT schema_name, view_name, definition, description
		FROM km_views WHERE database_name = ? AND schema_name = ?
		ORDER BY view_name`,
		dbName, schemaName,
	)
	return result, err
}

type FunctionRow struct {
	SchemaName   string         `json:"schema_name" db:"schema_name"`
	FunctionName string         `json:"function_name" db:"function_name"`
	ResultType   sql.NullString `json:"result_type" db:"result_type"`
	ArgTypes     sql.NullString `json:"argument_types" db:"argument_types"`
	Description  sql.NullString `json:"description" db:"description"`
	Language     sql.NullString `json:"language" db:"language"`
}

func (s *Store) ListFunctions(dbName, schemaName string) ([]FunctionRow, error) {
	var result []FunctionRow
	err := s.db.Select(&result,
		`SELECT schema_name, function_name, result_type, argument_types, description, language
		FROM km_functions WHERE database_name = ? AND schema_name = ?
		ORDER BY function_name`,
		dbName, schemaName,
	)
	return result, err
}

type SearchResult struct {
	DatabaseName string `json:"database_name" db:"database_name"`
	SchemaName   string `json:"schema_name" db:"schema_name"`
	ObjectType   string `json:"object_type" db:"object_type"`
	ObjectName   string `json:"object_name" db:"object_name"`
	Detail       string `json:"detail" db:"detail"`
}

// sanitizeFTSQuery wraps each token in double quotes to prevent FTS5 syntax
// errors from user input. This treats the input as a plain keyword search.
func sanitizeFTSQuery(query string) string {
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return ""
	}
	for i, t := range tokens {
		// Remove any existing double quotes to avoid injection
		t = strings.ReplaceAll(t, `"`, ``)
		tokens[i] = `"` + t + `"`
	}
	return strings.Join(tokens, " ")
}

func (s *Store) SearchSchema(query string) ([]SearchResult, error) {
	sanitized := sanitizeFTSQuery(query)
	if sanitized == "" {
		return nil, fmt.Errorf("empty search query")
	}

	var result []SearchResult
	err := s.db.Select(&result,
		`SELECT database_name, schema_name, object_type, object_name, detail
		FROM km_search WHERE km_search MATCH ?
		ORDER BY rank LIMIT 50`,
		sanitized,
	)
	return result, err
}
