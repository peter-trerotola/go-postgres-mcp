package guard

import "testing"

// =============================================================================
// Adversarial Guard Tests
// =============================================================================
//
// This file tests the SQL guard (Tier 1: AST validation) against adversarial
// inputs designed to bypass read-only enforcement. The guard uses PostgreSQL's
// actual parser (pg_query_go / libpg_query) to structurally validate SQL.
//
// Defense layers:
//   Tier 1 - AST validation (this guard) — blocks non-SELECT statement types
//   Tier 2 - Connection-level default_transaction_read_only=on
//   Tier 3 - Transaction-level BEGIN READ ONLY
//   Tier 4 - PostgreSQL user grants (SELECT only)
//
// Tests are organized as table-driven slices so adding new cases is just
// appending a struct. Each test case has:
//   - name:  short description of the technique
//   - sql:   the SQL to test
//   - tier:  which defense tier is expected to catch it (for documentation)
//
// CONTRIBUTING
//
// To add a new adversarial test case:
//   1. Add an entry to the appropriate slice (mustBlock or allowedButHarmless).
//   2. Pick a descriptive name and include the attack technique.
//   3. Set `tier` to document which defense tier catches it:
//        "tier1" = AST guard blocks it (parse error or non-SELECT node)
//        "tier2" = Connection-level read-only catches it at runtime
//        "tier3" = Transaction-level READ ONLY catches it at runtime
//        "tier4" = PostgreSQL user grants prevent execution
//   4. Run: go test ./internal/guard/ -run TestAdversarial -v
//
// If you find a query that bypasses ALL tiers, please open a security issue.
// =============================================================================

// adversarialCase is a test case for adversarial SQL inputs.
// The `tier` field documents which defense layer is expected to block the query.
type adversarialCase struct {
	name string // short description of the technique
	sql  string // the SQL to test
	tier string // which tier catches this: "tier1", "tier2", "tier3", "tier4"
}

// TestAdversarial_MustBlock tests SQL that the AST guard (Tier 1) MUST reject.
// These are queries that contain non-SELECT statement types, wrapped or
// disguised in various ways.
func TestAdversarial_MustBlock(t *testing.T) {
	mustBlock := []adversarialCase{

		// --- Statement smuggling via semicolons ---
		{"semicolon INSERT after SELECT", "SELECT 1; INSERT INTO users VALUES (1)", "tier1"},
		{"semicolon DROP after SELECT", "SELECT 1; DROP TABLE users", "tier1"},
		{"semicolon UPDATE after SELECT", "SELECT 1; UPDATE users SET name='x'", "tier1"},
		{"semicolon DELETE after SELECT", "SELECT 1; DELETE FROM users", "tier1"},
		{"three statements with middle mutation", "SELECT 1; UPDATE users SET x=1; SELECT 2", "tier1"},
		{"semicolon TRUNCATE after SELECT", "SELECT 1; TRUNCATE users", "tier1"},
		{"semicolon CREATE TABLE after SELECT", "SELECT 1; CREATE TABLE evil(id int)", "tier1"},

		// --- CTE mutation smuggling ---
		{"CTE with INSERT RETURNING", "WITH x AS (INSERT INTO users(name) VALUES('evil') RETURNING id) SELECT * FROM x", "tier1"},
		{"CTE with UPDATE RETURNING", "WITH x AS (UPDATE users SET name='evil' RETURNING id) SELECT * FROM x", "tier1"},
		{"CTE with DELETE RETURNING", "WITH x AS (DELETE FROM users RETURNING id) SELECT * FROM x", "tier1"},
		{"nested CTE mutation", "WITH a AS (WITH b AS (INSERT INTO users(name) VALUES('x') RETURNING id) SELECT * FROM b) SELECT * FROM a", "tier1"},
		{"CTE with TRUNCATE", "WITH x AS (TRUNCATE users RETURNING *) SELECT 1", "tier1"},

		// --- SELECT INTO (creates table) ---
		{"SELECT INTO basic", "SELECT * INTO newtable FROM users", "tier1"},
		{"SELECT INTO with schema", "SELECT * INTO public.newtable FROM users", "tier1"},
		{"SELECT INTO TEMP", "SELECT * INTO TEMP newtable FROM users", "tier1"},
		{"SELECT INTO TEMPORARY", "SELECT * INTO TEMPORARY newtable FROM users", "tier1"},
		{"SELECT INTO UNLOGGED", "SELECT * INTO UNLOGGED newtable FROM users", "tier1"},
		{"UNION right branch INTO", "SELECT 1 UNION SELECT * INTO newtable FROM users", "tier1"},

		// --- Locking clauses ---
		{"FOR UPDATE", "SELECT * FROM users FOR UPDATE", "tier1"},
		{"FOR NO KEY UPDATE", "SELECT * FROM users FOR NO KEY UPDATE", "tier1"},
		{"FOR SHARE", "SELECT * FROM users FOR SHARE", "tier1"},
		{"FOR KEY SHARE", "SELECT * FROM users FOR KEY SHARE", "tier1"},
		{"FOR UPDATE SKIP LOCKED", "SELECT * FROM users FOR UPDATE SKIP LOCKED", "tier1"},
		{"FOR UPDATE NOWAIT", "SELECT * FROM users FOR UPDATE NOWAIT", "tier1"},
		{"FOR UPDATE OF specific table", "SELECT * FROM users u JOIN orders o ON u.id=o.uid FOR UPDATE OF u", "tier1"},
		{"subquery with FOR UPDATE in CTE", "WITH locked AS (SELECT * FROM users FOR UPDATE) SELECT * FROM locked", "tier1"},

		// --- EXPLAIN with mutations ---
		{"EXPLAIN INSERT", "EXPLAIN INSERT INTO users(name) VALUES('x')", "tier1"},
		{"EXPLAIN UPDATE", "EXPLAIN UPDATE users SET name='x'", "tier1"},
		{"EXPLAIN DELETE", "EXPLAIN DELETE FROM users", "tier1"},
		{"EXPLAIN ANALYZE INSERT", "EXPLAIN ANALYZE INSERT INTO users(name) VALUES('x')", "tier1"},
		{"EXPLAIN ANALYZE UPDATE", "EXPLAIN ANALYZE UPDATE users SET name='x'", "tier1"},
		{"EXPLAIN ANALYZE DELETE", "EXPLAIN ANALYZE DELETE FROM users WHERE id=1", "tier1"},
		{"EXPLAIN CREATE TABLE AS", "EXPLAIN CREATE TABLE newtable AS SELECT * FROM users", "tier1"},

		// --- Transaction control ---
		{"BEGIN", "BEGIN", "tier1"},
		{"COMMIT", "COMMIT", "tier1"},
		{"ROLLBACK", "ROLLBACK", "tier1"},
		{"SAVEPOINT", "SAVEPOINT my_savepoint", "tier1"},
		{"RELEASE SAVEPOINT", "RELEASE SAVEPOINT my_savepoint", "tier1"},
		{"ROLLBACK TO SAVEPOINT", "ROLLBACK TO SAVEPOINT my_savepoint", "tier1"},
		{"BEGIN followed by SELECT", "BEGIN; SELECT 1", "tier1"},
		{"SELECT followed by COMMIT", "SELECT 1; COMMIT", "tier1"},

		// --- Prepared statements ---
		{"PREPARE", "PREPARE stmt AS SELECT 1", "tier1"},
		{"EXECUTE", "EXECUTE stmt", "tier1"},
		{"DEALLOCATE", "DEALLOCATE stmt", "tier1"},
		{"DEALLOCATE ALL", "DEALLOCATE ALL", "tier1"},
		{"PREPARE INSERT", "PREPARE ins AS INSERT INTO users(name) VALUES($1)", "tier1"},

		// --- DDL statements ---
		{"CREATE TABLE", "CREATE TABLE evil(id int)", "tier1"},
		{"CREATE TABLE AS SELECT", "CREATE TABLE evil AS SELECT * FROM users", "tier1"},
		{"CREATE VIEW", "CREATE VIEW evil AS SELECT * FROM users", "tier1"},
		{"CREATE MATERIALIZED VIEW", "CREATE MATERIALIZED VIEW evil AS SELECT * FROM users", "tier1"},
		{"CREATE INDEX", "CREATE INDEX evil_idx ON users(name)", "tier1"},
		{"CREATE FUNCTION", "CREATE FUNCTION evil() RETURNS void AS $$ BEGIN END $$ LANGUAGE plpgsql", "tier1"},
		{"CREATE TRIGGER", "CREATE TRIGGER evil AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION evil()", "tier1"},
		{"CREATE RULE", "CREATE RULE evil AS ON SELECT TO users DO INSTEAD NOTHING", "tier1"},
		{"CREATE EXTENSION", "CREATE EXTENSION dblink", "tier1"},
		{"CREATE SCHEMA", "CREATE SCHEMA evil", "tier1"},
		{"CREATE TYPE", "CREATE TYPE mood AS ENUM ('happy', 'sad')", "tier1"},
		{"CREATE SEQUENCE", "CREATE SEQUENCE evil_seq", "tier1"},
		{"CREATE DATABASE", "CREATE DATABASE evil", "tier1"},
		{"CREATE ROLE", "CREATE ROLE evil", "tier1"},
		{"CREATE USER", "CREATE USER evil", "tier1"},

		// --- ALTER statements ---
		{"ALTER TABLE", "ALTER TABLE users ADD COLUMN evil text", "tier1"},
		{"ALTER TABLE RENAME", "ALTER TABLE users RENAME TO evil", "tier1"},
		{"ALTER TABLE DROP COLUMN", "ALTER TABLE users DROP COLUMN name", "tier1"},
		{"ALTER VIEW", "ALTER VIEW myview RENAME TO evil", "tier1"},
		{"ALTER FUNCTION", "ALTER FUNCTION myfn() RENAME TO evil", "tier1"},
		{"ALTER SEQUENCE", "ALTER SEQUENCE my_seq RESTART WITH 1", "tier1"},
		{"ALTER ROLE", "ALTER ROLE readonly PASSWORD 'hacked'", "tier1"},
		{"ALTER DATABASE", "ALTER DATABASE mydb SET work_mem='1GB'", "tier1"},
		{"ALTER DEFAULT PRIVILEGES", "ALTER DEFAULT PRIVILEGES GRANT ALL ON TABLES TO evil", "tier1"},

		// --- DROP statements ---
		{"DROP TABLE", "DROP TABLE users", "tier1"},
		{"DROP TABLE CASCADE", "DROP TABLE users CASCADE", "tier1"},
		{"DROP TABLE IF EXISTS", "DROP TABLE IF EXISTS users", "tier1"},
		{"DROP VIEW", "DROP VIEW myview", "tier1"},
		{"DROP INDEX", "DROP INDEX users_pkey", "tier1"},
		{"DROP FUNCTION", "DROP FUNCTION myfn()", "tier1"},
		{"DROP SCHEMA", "DROP SCHEMA public CASCADE", "tier1"},
		{"DROP DATABASE", "DROP DATABASE mydb", "tier1"},
		{"DROP EXTENSION", "DROP EXTENSION dblink", "tier1"},
		{"DROP ROLE", "DROP ROLE readonly", "tier1"},
		{"DROP SEQUENCE", "DROP SEQUENCE my_seq", "tier1"},
		{"DROP TRIGGER", "DROP TRIGGER evil ON users", "tier1"},
		{"DROP RULE", "DROP RULE evil ON users", "tier1"},

		// --- DML mutations ---
		{"INSERT", "INSERT INTO users(name) VALUES('evil')", "tier1"},
		{"INSERT with ON CONFLICT", "INSERT INTO users(id, name) VALUES(1, 'evil') ON CONFLICT DO NOTHING", "tier1"},
		{"INSERT from SELECT", "INSERT INTO evil SELECT * FROM users", "tier1"},
		{"UPDATE", "UPDATE users SET name='evil' WHERE id=1", "tier1"},
		{"UPDATE with FROM", "UPDATE users SET name=e.name FROM evil e WHERE users.id=e.id", "tier1"},
		{"UPDATE with RETURNING", "UPDATE users SET name='evil' RETURNING *", "tier1"},
		{"DELETE", "DELETE FROM users WHERE id=1", "tier1"},
		{"DELETE with USING", "DELETE FROM users USING evil WHERE users.id=evil.id", "tier1"},
		{"MERGE", "MERGE INTO users USING source ON users.id=source.id WHEN MATCHED THEN UPDATE SET name='x'", "tier1"},
		{"TRUNCATE", "TRUNCATE users", "tier1"},
		{"TRUNCATE CASCADE", "TRUNCATE users CASCADE", "tier1"},

		// --- COPY ---
		{"COPY TO stdout", "COPY users TO STDOUT", "tier1"},
		{"COPY TO file", "COPY users TO '/tmp/evil.csv'", "tier1"},
		{"COPY FROM stdin", "COPY users FROM STDIN", "tier1"},
		{"COPY with query", "COPY (SELECT * FROM users) TO STDOUT", "tier1"},

		// --- Session/config manipulation ---
		{"SET", "SET work_mem='1GB'", "tier1"},
		{"SET LOCAL", "SET LOCAL work_mem='1GB'", "tier1"},
		{"SET SESSION", "SET SESSION work_mem='1GB'", "tier1"},
		{"RESET", "RESET work_mem", "tier1"},
		{"RESET ALL", "RESET ALL", "tier1"},
		{"DISCARD", "DISCARD ALL", "tier1"},
		{"DISCARD PLANS", "DISCARD PLANS", "tier1"},
		{"DISCARD SEQUENCES", "DISCARD SEQUENCES", "tier1"},
		{"DISCARD TEMP", "DISCARD TEMP", "tier1"},

		// --- Administrative commands ---
		{"VACUUM", "VACUUM users", "tier1"},
		{"VACUUM FULL", "VACUUM FULL users", "tier1"},
		{"ANALYZE", "ANALYZE users", "tier1"},
		{"REINDEX", "REINDEX TABLE users", "tier1"},
		{"CLUSTER", "CLUSTER users USING users_pkey", "tier1"},
		{"REFRESH MATERIALIZED VIEW", "REFRESH MATERIALIZED VIEW myview", "tier1"},

		// --- Privilege manipulation ---
		{"GRANT SELECT", "GRANT SELECT ON users TO evil", "tier1"},
		{"GRANT ALL", "GRANT ALL ON users TO evil", "tier1"},
		{"REVOKE", "REVOKE SELECT ON users FROM readonly", "tier1"},
		{"GRANT EXECUTE", "GRANT EXECUTE ON FUNCTION myfn() TO evil", "tier1"},
		{"GRANT USAGE ON SCHEMA", "GRANT USAGE ON SCHEMA public TO evil", "tier1"},
		{"GRANT CONNECT", "GRANT CONNECT ON DATABASE mydb TO evil", "tier1"},

		// --- Notification/listen ---
		{"LISTEN", "LISTEN my_channel", "tier1"},
		{"UNLISTEN", "UNLISTEN my_channel", "tier1"},
		{"NOTIFY", "NOTIFY my_channel", "tier1"},
		{"NOTIFY with payload", "NOTIFY my_channel, 'payload'", "tier1"},

		// --- Cursor ---
		{"DECLARE CURSOR", "DECLARE mycursor CURSOR FOR SELECT * FROM users", "tier1"},
		{"FETCH", "FETCH NEXT FROM mycursor", "tier1"},
		{"CLOSE cursor", "CLOSE mycursor", "tier1"},
		{"MOVE cursor", "MOVE NEXT FROM mycursor", "tier1"},

		// --- LOCK ---
		{"LOCK TABLE", "LOCK TABLE users IN ACCESS EXCLUSIVE MODE", "tier1"},
		{"LOCK TABLE SHARE", "LOCK TABLE users IN SHARE MODE", "tier1"},
		{"LOCK TABLE NOWAIT", "LOCK TABLE users IN ACCESS EXCLUSIVE MODE NOWAIT", "tier1"},

		// --- Procedural ---
		{"DO block", "DO $$ BEGIN RAISE NOTICE 'evil'; END $$", "tier1"},
		{"DO block with mutation", "DO $$ BEGIN DELETE FROM users; END $$", "tier1"},
		{"CALL procedure", "CALL my_procedure()", "tier1"},
		{"CALL with args", "CALL my_procedure(1, 'evil')", "tier1"},

		// --- SECURITY LABEL / COMMENT (can change metadata) ---
		{"COMMENT ON TABLE", "COMMENT ON TABLE users IS 'evil'", "tier1"},
		{"COMMENT ON COLUMN", "COMMENT ON COLUMN users.name IS 'evil'", "tier1"},
		{"SECURITY LABEL", "SECURITY LABEL ON TABLE users IS 'evil'", "tier1"},

		// --- Encoding / whitespace tricks ---
		{"tab before DROP", "\tDROP TABLE users", "tier1"},
		{"newline before DELETE", "\nDELETE FROM users", "tier1"},
		{"carriage return before INSERT", "\rINSERT INTO users VALUES(1)", "tier1"},
		{"mixed whitespace before UPDATE", "  \t\n  UPDATE users SET x=1", "tier1"},
		{"lowercase mutation", "insert into users(name) values('evil')", "tier1"},
		{"mixed case mutation", "Insert Into users(name) Values('evil')", "tier1"},
		{"UPPER CASE mutation", "INSERT INTO USERS(NAME) VALUES('EVIL')", "tier1"},
	}

	for _, tc := range mustBlock {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.sql)
			if err == nil {
				t.Errorf("[%s] expected query to be BLOCKED, but it was allowed\n  SQL: %s", tc.tier, tc.sql)
			}
		})
	}
}

// TestAdversarial_FunctionCalls tests SELECT queries that call potentially
// dangerous functions. These are valid SELECT statements that pass Tier 1
// (AST validation) because the guard validates statement structure, not
// function semantics. They are expected to be caught by Tiers 2-4.
//
// This test documents the attack surface and verifies that legitimate-looking
// function calls in SELECT are not accidentally blocked by the AST guard.
func TestAdversarial_FunctionCalls(t *testing.T) {
	// These are syntactically valid SELECTs. The guard SHOULD allow them
	// because Tier 1 only validates statement types, not function names.
	// Defense against these relies on Tiers 2-4 (read-only transaction,
	// connection settings, user grants).
	functionCalls := []adversarialCase{
		// --- Side-effect functions (caught by Tier 2/3: read-only transaction) ---
		{"set_config", "SELECT set_config('work_mem', '1GB', false)", "tier2"},
		{"nextval (advances sequence)", "SELECT nextval('users_id_seq')", "tier2"},
		{"setval (resets sequence)", "SELECT setval('users_id_seq', 1)", "tier2"},

		// --- Large object functions (caught by Tier 2/3) ---
		{"lo_create", "SELECT lo_create(0)", "tier2"},
		{"lo_import", "SELECT lo_import('/etc/passwd')", "tier2"},
		{"lo_unlink", "SELECT lo_unlink(12345)", "tier2"},

		// --- File I/O (requires superuser, caught by Tier 4) ---
		{"pg_read_file", "SELECT pg_read_file('/etc/passwd')", "tier4"},
		{"pg_read_binary_file", "SELECT pg_read_binary_file('/etc/passwd')", "tier4"},

		// --- Connection killing (caught by Tier 4: needs pg_signal_backend) ---
		{"pg_terminate_backend", "SELECT pg_terminate_backend(1234)", "tier4"},
		{"pg_cancel_backend", "SELECT pg_cancel_backend(1234)", "tier4"},

		// --- DoS via resource consumption ---
		{"pg_sleep", "SELECT pg_sleep(3600)", "tier2"},
		{"pg_sleep in subquery", "SELECT * FROM users WHERE pg_sleep(1) IS NOT NULL", "tier2"},
		{"generate_series large", "SELECT * FROM generate_series(1, 1000000000)", "tier2"},
		{"recursive CTE infinite", "WITH RECURSIVE inf AS (SELECT 1 AS x UNION ALL SELECT x+1 FROM inf) SELECT * FROM inf", "tier2"},

		// --- Advisory locks (can deadlock, caught by Tier 2/3) ---
		{"pg_advisory_lock", "SELECT pg_advisory_lock(1)", "tier2"},
		{"pg_advisory_xact_lock", "SELECT pg_advisory_xact_lock(1)", "tier2"},
		{"pg_try_advisory_lock", "SELECT pg_try_advisory_lock(1)", "tier2"},

		// --- Notification (caught by Tier 2/3) ---
		{"pg_notify function", "SELECT pg_notify('channel', 'payload')", "tier2"},

		// --- Extension functions (require extension + grants, caught by Tier 4) ---
		{"dblink_exec", "SELECT dblink_exec('host=localhost', 'DROP TABLE users')", "tier4"},
		{"dblink query", "SELECT * FROM dblink('host=localhost', 'SELECT 1') AS t(id int)", "tier4"},

		// --- Information disclosure (caught by Tier 4: user grants) ---
		{"pg_authid read", "SELECT rolname, rolpassword FROM pg_authid", "tier4"},
		{"pg_shadow read", "SELECT usename, passwd FROM pg_shadow", "tier4"},
		{"pg_stat_activity", "SELECT * FROM pg_stat_activity", "tier4"},
		{"pg_stat_ssl", "SELECT * FROM pg_stat_ssl", "tier4"},
		{"pg_hba_file_rules", "SELECT * FROM pg_hba_file_rules", "tier4"},
		{"pg_file_settings", "SELECT * FROM pg_file_settings", "tier4"},

		// --- EXPLAIN ANALYZE with function side effects ---
		{"EXPLAIN ANALYZE with pg_sleep", "EXPLAIN ANALYZE SELECT pg_sleep(1)", "tier2"},
		{"EXPLAIN ANALYZE with nextval", "EXPLAIN ANALYZE SELECT nextval('users_id_seq')", "tier2"},

		// --- LATERAL with side-effect functions ---
		{"LATERAL with pg_terminate_backend", "SELECT * FROM pg_stat_activity, LATERAL (SELECT pg_terminate_backend(pid)) s", "tier4"},
		{"LATERAL with set_config", "SELECT * FROM users, LATERAL (SELECT set_config('work_mem', '1GB', false)) s", "tier2"},

		// --- Current user / version info (harmless but documents surface) ---
		{"current_user", "SELECT current_user", "tier4"},
		{"version", "SELECT version()", "tier4"},
		{"inet_server_addr", "SELECT inet_server_addr()", "tier4"},
		{"inet_server_port", "SELECT inet_server_port()", "tier4"},
	}

	for _, tc := range functionCalls {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.sql)
			if err != nil {
				t.Errorf("[%s] expected function call in SELECT to pass Tier 1 (caught by %s instead)\n  SQL: %s\n  Error: %v",
					tc.tier, tc.tier, tc.sql, err)
			}
		})
	}
}

// TestAdversarial_CommentAndEncoding tests that comment and encoding tricks
// cannot hide mutations from the parser.
func TestAdversarial_CommentAndEncoding(t *testing.T) {
	mustBlock := []adversarialCase{
		// --- Comments cannot hide mutations ---
		{"block comment before INSERT", "/* harmless */ INSERT INTO users VALUES(1)", "tier1"},
		{"block comment after SELECT hiding INSERT", "SELECT 1; /* comment */ INSERT INTO users VALUES(1)", "tier1"},
		{"nested block comments", "/* /* nested */ */ DROP TABLE users", "tier1"},
		{"line comment before DROP", "-- comment\nDROP TABLE users", "tier1"},
		{"line comment after SELECT", "SELECT 1;\n-- comment\nDELETE FROM users", "tier1"},

		// --- Dollar-quoted strings containing keywords ---
		// These parse correctly because the dollar-quoted content is a string literal
		// But if the outer statement is a mutation, it's still caught
		{"dollar-quote hiding DELETE", "DO $$ BEGIN DELETE FROM users; END $$", "tier1"},

		// --- Empty/garbage input ---
		{"empty string", "", "tier1"},
		{"only whitespace", "   \t\n  ", "tier1"},
		{"only comments", "-- just a comment", "tier1"},
		{"only block comment", "/* nothing here */", "tier1"},
	}

	// Null bytes: libpg_query (C) truncates at \x00 and only sees "SELECT 1".
	// PostgreSQL's wire protocol also truncates at null bytes, so the trailing
	// "; DROP TABLE users" never reaches the server. This is safe — not a bypass.

	for _, tc := range mustBlock {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.sql)
			if err == nil {
				t.Errorf("[%s] expected to be BLOCKED\n  SQL: %q", tc.tier, tc.sql)
			}
		})
	}

	// These contain scary-looking strings but are valid SELECTs
	allowedStrings := []adversarialCase{
		{"string literal with INSERT", "SELECT 'INSERT INTO users VALUES(1)'", "tier1"},
		{"string literal with DROP", "SELECT 'DROP TABLE users'", "tier1"},
		{"string literal with DELETE", "SELECT * FROM users WHERE name = 'DELETE FROM evil'", "tier1"},
		{"dollar-quoted string with DROP", "SELECT $$DROP TABLE users$$", "tier1"},
		{"dollar-quoted tagged string", "SELECT $tag$DELETE FROM users$tag$", "tier1"},
		{"E-string with escapes", "SELECT E'INSERT\\nINTO\\nusers'", "tier1"},
		{"concat of keywords", "SELECT 'DROP' || ' TABLE' || ' users'", "tier1"},
	}

	for _, tc := range allowedStrings {
		t.Run("allowed_"+tc.name, func(t *testing.T) {
			err := Validate(tc.sql)
			if err != nil {
				t.Errorf("expected string literal in SELECT to be ALLOWED\n  SQL: %s\n  Error: %v", tc.sql, err)
			}
		})
	}
}

// TestAdversarial_EdgeCases tests unusual but valid SQL constructs that should
// be correctly classified.
func TestAdversarial_EdgeCases(t *testing.T) {
	// Valid SELECTs that should NOT be blocked
	allowed := []adversarialCase{
		// --- VALUES clause (standalone) is a valid SELECT form ---
		{"VALUES clause", "VALUES (1, 'a'), (2, 'b')", "tier1"},
		{"VALUES with ORDER BY", "VALUES (1), (2) ORDER BY 1", "tier1"},

		// --- Deeply nested subqueries ---
		{"4-level nested subquery", "SELECT * FROM (SELECT * FROM (SELECT * FROM (SELECT 1) a) b) c", "tier1"},
		{"nested CTEs", "WITH a AS (SELECT 1), b AS (SELECT * FROM a), c AS (SELECT * FROM b) SELECT * FROM c", "tier1"},

		// --- Complex UNION trees ---
		{"triple UNION", "SELECT 1 UNION SELECT 2 UNION SELECT 3", "tier1"},
		{"mixed set ops", "SELECT 1 UNION SELECT 2 INTERSECT SELECT 2 EXCEPT SELECT 3", "tier1"},
		{"parenthesized set op", "(SELECT 1) UNION (SELECT 2)", "tier1"},

		// --- Table-valued functions (safe built-ins) ---
		{"generate_series", "SELECT * FROM generate_series(1, 10)", "tier1"},
		{"unnest", "SELECT * FROM unnest(ARRAY[1,2,3])", "tier1"},
		{"json_array_elements", "SELECT * FROM json_array_elements('[1,2,3]'::json)", "tier1"},
		{"jsonb_each", "SELECT * FROM jsonb_each('{\"a\":1}'::jsonb)", "tier1"},
		{"regexp_matches", "SELECT * FROM regexp_matches('hello world', '(\\w+)', 'g')", "tier1"},

		// --- SELECT with complex expressions ---
		{"CASE expression", "SELECT CASE WHEN 1=1 THEN 'yes' ELSE 'no' END", "tier1"},
		{"array constructor", "SELECT ARRAY[1, 2, 3]", "tier1"},
		{"row constructor", "SELECT ROW(1, 2, 3)", "tier1"},
		{"type cast chain", "SELECT '1'::int::text::int", "tier1"},

		// --- Quoted identifiers that look like keywords ---
		{"quoted table DELETE", `SELECT * FROM "DELETE"`, "tier1"},
		{"quoted table DROP", `SELECT * FROM "DROP"`, "tier1"},
		{"quoted column INSERT", `SELECT "INSERT" FROM users`, "tier1"},
		{"quoted schema.table", `SELECT * FROM "public"."INSERT"`, "tier1"},

		// --- Window functions ---
		{"row_number", "SELECT row_number() OVER () FROM users", "tier1"},
		{"dense_rank with partition", "SELECT dense_rank() OVER (PARTITION BY dept ORDER BY salary DESC) FROM emp", "tier1"},
		{"lag/lead", "SELECT lag(salary, 1) OVER (ORDER BY id) FROM emp", "tier1"},
		{"named window", "SELECT sum(x) OVER w FROM t WINDOW w AS (ORDER BY y)", "tier1"},

		// --- Multiple valid SELECTs ---
		{"two SELECTs", "SELECT 1; SELECT 2", "tier1"},
		{"three SELECTs", "SELECT 1; SELECT 2; SELECT 3", "tier1"},
		{"EXPLAIN followed by SELECT", "EXPLAIN SELECT 1; SELECT 2", "tier1"},
	}

	for _, tc := range allowed {
		t.Run("allowed_"+tc.name, func(t *testing.T) {
			err := Validate(tc.sql)
			if err != nil {
				t.Errorf("expected to be ALLOWED\n  SQL: %s\n  Error: %v", tc.sql, err)
			}
		})
	}

	// Mutations disguised within valid-looking constructs
	mustBlock := []adversarialCase{
		// --- UNION/INTERSECT with mutation in one branch ---
		{"UNION with INSERT in CTE", "WITH x AS (INSERT INTO users(name) VALUES('evil') RETURNING id) SELECT * FROM x UNION SELECT 1", "tier1"},
		{"INTERSECT with FOR UPDATE", "SELECT * FROM users FOR UPDATE INTERSECT SELECT * FROM users", "tier1"},

		// --- Complex CTE chains with mutation ---
		{"chained CTE with mutation", "WITH a AS (SELECT 1), b AS (DELETE FROM users RETURNING id) SELECT * FROM a", "tier1"},
		{"recursive CTE with mutation base", "WITH RECURSIVE r AS (INSERT INTO users(name) VALUES('x') RETURNING id UNION ALL SELECT id+1 FROM r WHERE id < 10) SELECT * FROM r", "tier1"},

		// --- SELECT INTO hidden in complex queries ---
		{"subquery with INTO (UNION right)", "(SELECT 1) UNION (SELECT * INTO evil FROM users)", "tier1"},
		{"CTE body with SELECT INTO", "WITH x AS (SELECT * INTO evil FROM users) SELECT 1", "tier1"},

		// --- FOR UPDATE hidden in complex queries ---
		{"CTE body with FOR UPDATE", "WITH x AS (SELECT * FROM users FOR UPDATE) SELECT * FROM x", "tier1"},
		{"subquery FOR UPDATE in UNION left", "(SELECT * FROM users FOR UPDATE) UNION (SELECT * FROM users)", "tier1"},
	}

	for _, tc := range mustBlock {
		t.Run("blocked_"+tc.name, func(t *testing.T) {
			err := Validate(tc.sql)
			if err == nil {
				t.Errorf("[%s] expected to be BLOCKED\n  SQL: %s", tc.tier, tc.sql)
			}
		})
	}
}
