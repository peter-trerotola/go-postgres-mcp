package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/petros/go-postgres-mcp/internal/config"
	"github.com/petros/go-postgres-mcp/internal/knowledgemap"
)

// Discover crawls a PostgreSQL database schema and populates the knowledge map.
// All existing knowledge map data for the database is cleared and then re-inserted.
// Schema and table filters from the config are applied during discovery.
//
// Queries run sequentially within a single read-only transaction, with SQLite
// writes interleaved after each query (pipeline pattern). This is faster than
// fetching everything first because PG I/O and SQLite writes overlap.
// Cross-database parallelism is handled by the caller (server.Start).
func Discover(ctx context.Context, pool *pgxpool.Pool, dbCfg config.DatabaseConfig, store *knowledgemap.Store) error {
	if err := store.ClearDatabase(dbCfg.Name); err != nil {
		return fmt.Errorf("clearing database %q: %w", dbCfg.Name, err)
	}

	if err := store.InsertDatabase(dbCfg.Name, dbCfg.Host, dbCfg.Port, dbCfg.Database); err != nil {
		return fmt.Errorf("inserting database record: %w", err)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("beginning read-only transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Schemas
	if err := discoverSchemas(ctx, tx, dbCfg, store); err != nil {
		return err
	}

	// Tables
	if err := discoverTables(ctx, tx, dbCfg, store); err != nil {
		return err
	}

	// Columns
	if err := discoverColumns(ctx, tx, dbCfg, store); err != nil {
		return err
	}

	// Constraints
	if err := discoverConstraints(ctx, tx, dbCfg, store); err != nil {
		return err
	}

	// Indexes
	if err := discoverIndexes(ctx, tx, dbCfg, store); err != nil {
		return err
	}

	// Foreign keys
	if err := discoverForeignKeys(ctx, tx, dbCfg, store); err != nil {
		return err
	}

	// Views
	if err := discoverViews(ctx, tx, dbCfg, store); err != nil {
		return err
	}

	// Functions
	if err := discoverFunctions(ctx, tx, dbCfg, store); err != nil {
		return err
	}

	if err := store.IndexForSearch(dbCfg.Name); err != nil {
		return fmt.Errorf("building search index: %w", err)
	}

	return nil
}

func discoverSchemas(ctx context.Context, tx pgx.Tx, dbCfg config.DatabaseConfig, store *knowledgemap.Store) error {
	rows, err := tx.Query(ctx, `
		SELECT schema_name FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_toast', 'pg_catalog', 'information_schema')
		ORDER BY schema_name`)
	if err != nil {
		return fmt.Errorf("querying schemas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return err
		}
		if !dbCfg.ShouldIncludeSchema(s) {
			continue
		}
		if err := store.InsertSchema(dbCfg.Name, s); err != nil {
			return fmt.Errorf("inserting schema %q: %w", s, err)
		}
	}
	return rows.Err()
}

func discoverTables(ctx context.Context, tx pgx.Tx, dbCfg config.DatabaseConfig, store *knowledgemap.Store) error {
	rows, err := tx.Query(ctx, `
		SELECT
			t.table_schema,
			t.table_name,
			t.table_type,
			COALESCE(c.reltuples::bigint, 0) AS row_estimate,
			COALESCE(pg_total_relation_size(c.oid), 0) AS size_bytes,
			COALESCE(obj_description(c.oid), '') AS description
		FROM information_schema.tables t
		LEFT JOIN pg_catalog.pg_namespace n ON n.nspname = t.table_schema
		LEFT JOIN pg_catalog.pg_class c ON c.relname = t.table_name AND c.relnamespace = n.oid
		WHERE t.table_schema NOT IN ('pg_toast', 'pg_catalog', 'information_schema')
		  AND t.table_type = 'BASE TABLE'
		ORDER BY t.table_schema, t.table_name`)
	if err != nil {
		return fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t knowledgemap.TableInfo
		if err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.SizeBytes, &t.Description); err != nil {
			return err
		}
		if !dbCfg.ShouldIncludeSchema(t.SchemaName) || !dbCfg.ShouldIncludeTable(t.SchemaName, t.TableName) {
			continue
		}
		if err := store.InsertTable(dbCfg.Name, t); err != nil {
			return err
		}
	}
	return rows.Err()
}

func discoverColumns(ctx context.Context, tx pgx.Tx, dbCfg config.DatabaseConfig, store *knowledgemap.Store) error {
	rows, err := tx.Query(ctx, `
		SELECT
			c.table_schema,
			c.table_name,
			c.column_name,
			c.ordinal_position,
			c.data_type,
			c.is_nullable = 'YES' AS is_nullable,
			c.column_default,
			COALESCE(col_description(
				(quote_ident(c.table_schema) || '.' || quote_ident(c.table_name))::regclass,
				c.ordinal_position
			), '') AS description
		FROM information_schema.columns c
		JOIN information_schema.tables t
			ON t.table_schema = c.table_schema AND t.table_name = c.table_name AND t.table_type = 'BASE TABLE'
		WHERE c.table_schema NOT IN ('pg_toast', 'pg_catalog', 'information_schema')
		ORDER BY c.table_schema, c.table_name, c.ordinal_position`)
	if err != nil {
		return fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var col knowledgemap.ColumnInfo
		if err := rows.Scan(&col.SchemaName, &col.TableName, &col.ColumnName,
			&col.Ordinal, &col.DataType, &col.IsNullable, &col.ColumnDefault, &col.Description); err != nil {
			return err
		}
		if !dbCfg.ShouldIncludeSchema(col.SchemaName) || !dbCfg.ShouldIncludeTable(col.SchemaName, col.TableName) {
			continue
		}
		if err := store.InsertColumn(dbCfg.Name, col); err != nil {
			return err
		}
	}
	return rows.Err()
}

func discoverConstraints(ctx context.Context, tx pgx.Tx, dbCfg config.DatabaseConfig, store *knowledgemap.Store) error {
	rows, err := tx.Query(ctx, `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			con.conname AS constraint_name,
			CASE con.contype
				WHEN 'p' THEN 'PRIMARY KEY'
				WHEN 'u' THEN 'UNIQUE'
				WHEN 'c' THEN 'CHECK'
				WHEN 'f' THEN 'FOREIGN KEY'
				WHEN 'x' THEN 'EXCLUSION'
			END AS constraint_type,
			pg_get_constraintdef(con.oid) AS definition
		FROM pg_catalog.pg_constraint con
		JOIN pg_catalog.pg_class c ON c.oid = con.conrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname NOT IN ('pg_toast', 'pg_catalog', 'information_schema')
		  AND c.relkind <> 'm'
		ORDER BY n.nspname, c.relname, con.conname`)
	if err != nil {
		return fmt.Errorf("querying constraints: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var con knowledgemap.ConstraintInfo
		if err := rows.Scan(&con.SchemaName, &con.TableName, &con.ConstraintName, &con.ConstraintType, &con.Definition); err != nil {
			return err
		}
		if !dbCfg.ShouldIncludeSchema(con.SchemaName) || !dbCfg.ShouldIncludeTable(con.SchemaName, con.TableName) {
			continue
		}
		if err := store.InsertConstraint(dbCfg.Name, con); err != nil {
			return err
		}
	}
	return rows.Err()
}

func discoverIndexes(ctx context.Context, tx pgx.Tx, dbCfg config.DatabaseConfig, store *knowledgemap.Store) error {
	rows, err := tx.Query(ctx, `
		SELECT
			n.nspname AS schema_name,
			t.relname AS table_name,
			i.relname AS index_name,
			ix.indisunique AS is_unique,
			ix.indisprimary AS is_primary,
			pg_get_indexdef(ix.indexrelid) AS definition
		FROM pg_catalog.pg_index ix
		JOIN pg_catalog.pg_class t ON t.oid = ix.indrelid
		JOIN pg_catalog.pg_class i ON i.oid = ix.indexrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname NOT IN ('pg_toast', 'pg_catalog', 'information_schema')
		  AND t.relkind <> 'm'
		ORDER BY n.nspname, t.relname, i.relname`)
	if err != nil {
		return fmt.Errorf("querying indexes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var idx knowledgemap.IndexInfo
		if err := rows.Scan(&idx.SchemaName, &idx.TableName, &idx.IndexName, &idx.IsUnique, &idx.IsPrimary, &idx.Definition); err != nil {
			return err
		}
		if !dbCfg.ShouldIncludeSchema(idx.SchemaName) || !dbCfg.ShouldIncludeTable(idx.SchemaName, idx.TableName) {
			continue
		}
		if err := store.InsertIndex(dbCfg.Name, idx); err != nil {
			return err
		}
	}
	return rows.Err()
}

func discoverForeignKeys(ctx context.Context, tx pgx.Tx, dbCfg config.DatabaseConfig, store *knowledgemap.Store) error {
	rows, err := tx.Query(ctx, `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			con.conname AS constraint_name,
			a.attname AS column_name,
			rn.nspname AS ref_schema,
			rc.relname AS ref_table,
			ra.attname AS ref_column
		FROM pg_catalog.pg_constraint con
		JOIN pg_catalog.pg_class c ON c.oid = con.conrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_catalog.pg_class rc ON rc.oid = con.confrelid
		JOIN pg_catalog.pg_namespace rn ON rn.oid = rc.relnamespace
		CROSS JOIN LATERAL unnest(con.conkey, con.confkey) WITH ORDINALITY AS cols(conkey, confkey, ord)
		JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid AND a.attnum = cols.conkey
		JOIN pg_catalog.pg_attribute ra ON ra.attrelid = rc.oid AND ra.attnum = cols.confkey
		WHERE con.contype = 'f'
		  AND n.nspname NOT IN ('pg_toast', 'pg_catalog', 'information_schema')
		ORDER BY n.nspname, c.relname, con.conname, cols.ord`)
	if err != nil {
		return fmt.Errorf("querying foreign keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var fk knowledgemap.ForeignKeyInfo
		if err := rows.Scan(&fk.SchemaName, &fk.TableName, &fk.ConstraintName, &fk.ColumnName,
			&fk.RefSchema, &fk.RefTable, &fk.RefColumn); err != nil {
			return err
		}
		if !dbCfg.ShouldIncludeSchema(fk.SchemaName) || !dbCfg.ShouldIncludeTable(fk.SchemaName, fk.TableName) {
			continue
		}
		if err := store.InsertForeignKey(dbCfg.Name, fk); err != nil {
			return err
		}
	}
	return rows.Err()
}

func discoverViews(ctx context.Context, tx pgx.Tx, dbCfg config.DatabaseConfig, store *knowledgemap.Store) error {
	rows, err := tx.Query(ctx, `
		SELECT
			v.schemaname AS schema_name,
			v.viewname AS view_name,
			v.definition,
			COALESCE(obj_description((quote_ident(v.schemaname) || '.' || quote_ident(v.viewname))::regclass), '') AS description
		FROM pg_catalog.pg_views v
		WHERE v.schemaname NOT IN ('pg_toast', 'pg_catalog', 'information_schema')
		ORDER BY v.schemaname, v.viewname`)
	if err != nil {
		return fmt.Errorf("querying views: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var view knowledgemap.ViewInfo
		if err := rows.Scan(&view.SchemaName, &view.ViewName, &view.Definition, &view.Description); err != nil {
			return err
		}
		if !dbCfg.ShouldIncludeSchema(view.SchemaName) {
			continue
		}
		if err := store.InsertView(dbCfg.Name, view); err != nil {
			return err
		}
	}
	return rows.Err()
}

func discoverFunctions(ctx context.Context, tx pgx.Tx, dbCfg config.DatabaseConfig, store *knowledgemap.Store) error {
	rows, err := tx.Query(ctx, `
		SELECT
			n.nspname AS schema_name,
			p.proname AS function_name,
			pg_get_function_result(p.oid) AS result_type,
			pg_get_function_arguments(p.oid) AS argument_types,
			COALESCE(obj_description(p.oid, 'pg_proc'), '') AS description,
			l.lanname AS language
		FROM pg_catalog.pg_proc p
		JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
		JOIN pg_catalog.pg_language l ON l.oid = p.prolang
		WHERE n.nspname NOT IN ('pg_toast', 'pg_catalog', 'information_schema')
		  AND p.prokind IN ('f', 'w')
		ORDER BY n.nspname, p.proname`)
	if err != nil {
		return fmt.Errorf("querying functions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var fn knowledgemap.FunctionInfo
		if err := rows.Scan(&fn.SchemaName, &fn.FunctionName, &fn.ResultType, &fn.ArgTypes, &fn.Description, &fn.Language); err != nil {
			return err
		}
		if !dbCfg.ShouldIncludeSchema(fn.SchemaName) {
			continue
		}
		if err := store.InsertFunction(dbCfg.Name, fn); err != nil {
			return err
		}
	}
	return rows.Err()
}
