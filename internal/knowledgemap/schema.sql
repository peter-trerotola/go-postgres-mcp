CREATE TABLE IF NOT EXISTS km_databases (
	name         TEXT PRIMARY KEY,
	host         TEXT NOT NULL,
	port         INTEGER NOT NULL,
	database     TEXT NOT NULL,
	discovered_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS km_schemas (
	database_name TEXT NOT NULL,
	schema_name   TEXT NOT NULL,
	PRIMARY KEY (database_name, schema_name),
	FOREIGN KEY (database_name) REFERENCES km_databases(name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS km_tables (
	database_name TEXT NOT NULL,
	schema_name   TEXT NOT NULL,
	table_name    TEXT NOT NULL,
	table_type    TEXT NOT NULL DEFAULT 'BASE TABLE',
	row_estimate  INTEGER,
	size_bytes    INTEGER,
	description   TEXT,
	PRIMARY KEY (database_name, schema_name, table_name),
	FOREIGN KEY (database_name, schema_name) REFERENCES km_schemas(database_name, schema_name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS km_columns (
	database_name  TEXT NOT NULL,
	schema_name    TEXT NOT NULL,
	table_name     TEXT NOT NULL,
	column_name    TEXT NOT NULL,
	ordinal        INTEGER NOT NULL,
	data_type      TEXT NOT NULL,
	is_nullable    INTEGER NOT NULL DEFAULT 1,
	column_default TEXT,
	description    TEXT,
	PRIMARY KEY (database_name, schema_name, table_name, column_name),
	FOREIGN KEY (database_name, schema_name, table_name) REFERENCES km_tables(database_name, schema_name, table_name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS km_constraints (
	database_name   TEXT NOT NULL,
	schema_name     TEXT NOT NULL,
	table_name      TEXT NOT NULL,
	constraint_name TEXT NOT NULL,
	constraint_type TEXT NOT NULL,
	definition      TEXT,
	PRIMARY KEY (database_name, schema_name, table_name, constraint_name),
	FOREIGN KEY (database_name, schema_name, table_name) REFERENCES km_tables(database_name, schema_name, table_name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS km_indexes (
	database_name TEXT NOT NULL,
	schema_name   TEXT NOT NULL,
	table_name    TEXT NOT NULL,
	index_name    TEXT NOT NULL,
	is_unique     INTEGER NOT NULL DEFAULT 0,
	is_primary    INTEGER NOT NULL DEFAULT 0,
	definition    TEXT,
	PRIMARY KEY (database_name, schema_name, table_name, index_name),
	FOREIGN KEY (database_name, schema_name, table_name) REFERENCES km_tables(database_name, schema_name, table_name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS km_foreign_keys (
	database_name        TEXT NOT NULL,
	schema_name          TEXT NOT NULL,
	table_name           TEXT NOT NULL,
	constraint_name      TEXT NOT NULL,
	column_name          TEXT NOT NULL,
	ref_schema           TEXT NOT NULL,
	ref_table            TEXT NOT NULL,
	ref_column           TEXT NOT NULL,
	PRIMARY KEY (database_name, schema_name, table_name, constraint_name, column_name),
	FOREIGN KEY (database_name, schema_name, table_name) REFERENCES km_tables(database_name, schema_name, table_name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS km_views (
	database_name TEXT NOT NULL,
	schema_name   TEXT NOT NULL,
	view_name     TEXT NOT NULL,
	definition    TEXT,
	description   TEXT,
	PRIMARY KEY (database_name, schema_name, view_name),
	FOREIGN KEY (database_name, schema_name) REFERENCES km_schemas(database_name, schema_name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS km_functions (
	database_name  TEXT NOT NULL,
	schema_name    TEXT NOT NULL,
	function_name  TEXT NOT NULL,
	result_type    TEXT,
	argument_types TEXT,
	description    TEXT,
	language       TEXT,
	PRIMARY KEY (database_name, schema_name, function_name, argument_types),
	FOREIGN KEY (database_name, schema_name) REFERENCES km_schemas(database_name, schema_name) ON DELETE CASCADE
);

CREATE VIRTUAL TABLE IF NOT EXISTS km_search USING fts5(
	database_name,
	schema_name,
	object_type,
	object_name,
	detail
);
