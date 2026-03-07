package guard

import (
	"errors"
	"testing"
)

func TestAllowedQueries(t *testing.T) {
	allowed := []struct {
		name string
		sql  string
	}{
		{"simple select", "SELECT 1"},
		{"select from table", "SELECT * FROM users"},
		{"select with where", "SELECT id, name FROM users WHERE active = true"},
		{"select with join", "SELECT u.name, o.total FROM users u JOIN orders o ON u.id = o.user_id"},
		{"select with left join", "SELECT u.name, o.total FROM users u LEFT JOIN orders o ON u.id = o.user_id"},
		{"select with subquery", "SELECT * FROM users WHERE id IN (SELECT user_id FROM orders)"},
		{"select with CTE", "WITH active_users AS (SELECT * FROM users WHERE active = true) SELECT * FROM active_users"},
		{"select with aggregate", "SELECT COUNT(*), department FROM employees GROUP BY department"},
		{"select with UNION", "SELECT id FROM users UNION SELECT id FROM admins"},
		{"select with UNION ALL", "SELECT id FROM users UNION ALL SELECT id FROM admins"},
		{"select with INTERSECT", "SELECT id FROM users INTERSECT SELECT id FROM admins"},
		{"select with EXCEPT", "SELECT id FROM users EXCEPT SELECT id FROM banned"},
		{"select with LIMIT", "SELECT * FROM users LIMIT 10 OFFSET 20"},
		{"select with ORDER BY", "SELECT * FROM users ORDER BY created_at DESC"},
		{"select with HAVING", "SELECT department, COUNT(*) FROM employees GROUP BY department HAVING COUNT(*) > 5"},
		{"select with DISTINCT", "SELECT DISTINCT department FROM employees"},
		{"select with CASE", "SELECT CASE WHEN active THEN 'yes' ELSE 'no' END FROM users"},
		{"select with COALESCE", "SELECT COALESCE(name, 'unknown') FROM users"},
		{"select with cast", "SELECT CAST(id AS TEXT) FROM users"},
		{"select with string containing keyword", "SELECT * FROM users WHERE name = 'INSERT INTO test'"},
		{"EXPLAIN select", "EXPLAIN SELECT * FROM users"},
		{"EXPLAIN ANALYZE select", "EXPLAIN ANALYZE SELECT * FROM users"},
		{"select with window function", "SELECT id, ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary DESC) FROM employees"},
		{"select with LATERAL", "SELECT * FROM users u, LATERAL (SELECT * FROM orders o WHERE o.user_id = u.id LIMIT 1) sub"},
		{"select information_schema", "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'"},
		{"select pg_catalog", "SELECT relname FROM pg_catalog.pg_class"},
		{"nested CTEs", "WITH a AS (SELECT 1 AS x), b AS (SELECT x + 1 AS y FROM a) SELECT * FROM b"},
		{"recursive CTE", "WITH RECURSIVE t(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM t WHERE n < 100) SELECT n FROM t"},
		{"select with EXISTS", "SELECT * FROM users WHERE EXISTS (SELECT 1 FROM orders WHERE orders.user_id = users.id)"},
		{"select with NOT EXISTS", "SELECT * FROM users WHERE NOT EXISTS (SELECT 1 FROM orders WHERE orders.user_id = users.id)"},
		{"select with ANY", "SELECT * FROM users WHERE id = ANY(SELECT user_id FROM orders)"},
		{"select with ALL", "SELECT * FROM users WHERE salary > ALL(SELECT salary FROM interns)"},
		{"select with generate_series", "SELECT * FROM generate_series(1, 10)"},
		{"select with json functions", "SELECT jsonb_build_object('key', value) FROM config"},
		{"select with array operations", "SELECT array_agg(name ORDER BY name) FROM users GROUP BY department"},
		{"select with string_agg", "SELECT string_agg(name, ', ') FROM users"},
		{"select with date functions", "SELECT date_trunc('month', created_at), COUNT(*) FROM users GROUP BY 1"},
		{"select with type casting", "SELECT '2024-01-01'::date"},
		{"select boolean", "SELECT true, false"},
		{"select null", "SELECT NULL"},
		{"EXPLAIN VERBOSE select", "EXPLAIN VERBOSE SELECT * FROM users"},
		{"EXPLAIN with options", "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT * FROM users"},
		{"keyword in column alias", "SELECT id AS update_id FROM users"},
		{"keyword in table name", "SELECT * FROM update_log"},
		{"quoted identifier with keyword name", `SELECT * FROM "DELETE"`},
		{"keyword as quoted column alias", `SELECT id AS "update" FROM users`},
	}

	for _, tc := range allowed {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.sql); err != nil {
				t.Errorf("expected query to be allowed, got error: %v\n  SQL: %s", err, tc.sql)
			}
		})
	}
}

func TestBlockedQueries(t *testing.T) {
	blocked := []struct {
		name string
		sql  string
	}{
		{"INSERT", "INSERT INTO users (name) VALUES ('test')"},
		{"UPDATE", "UPDATE users SET name = 'test' WHERE id = 1"},
		{"DELETE", "DELETE FROM users WHERE id = 1"},
		{"DROP TABLE", "DROP TABLE users"},
		{"DROP DATABASE", "DROP DATABASE mydb"},
		{"ALTER TABLE", "ALTER TABLE users ADD COLUMN email TEXT"},
		{"CREATE TABLE", "CREATE TABLE test (id INT)"},
		{"CREATE INDEX", "CREATE INDEX idx ON users(name)"},
		{"TRUNCATE", "TRUNCATE users"},
		{"GRANT", "GRANT SELECT ON users TO readonly"},
		{"REVOKE", "REVOKE SELECT ON users FROM readonly"},
		{"COPY", "COPY users TO '/tmp/users.csv'"},
		{"SELECT INTO", "SELECT * INTO new_table FROM users"},
		{"CTE with INSERT", "WITH ins AS (INSERT INTO users (name) VALUES ('test') RETURNING id) SELECT * FROM ins"},
		{"CTE with UPDATE", "WITH upd AS (UPDATE users SET name = 'new' RETURNING id) SELECT * FROM upd"},
		{"CTE with DELETE", "WITH del AS (DELETE FROM users RETURNING id) SELECT * FROM del"},
		{"DO block", "DO $$ BEGIN RAISE NOTICE 'hello'; END $$"},
		{"CALL procedure", "CALL my_procedure()"},
		{"SET", "SET work_mem = '256MB'"},
		{"VACUUM", "VACUUM users"},
		{"LOCK TABLE", "LOCK TABLE users IN ACCESS EXCLUSIVE MODE"},
		{"multiple statements with mutation", "SELECT 1; DROP TABLE users"},
		{"EXPLAIN INSERT", "EXPLAIN INSERT INTO users (name) VALUES ('test')"},
		{"SELECT FOR UPDATE", "SELECT * FROM users FOR UPDATE"},
		{"SELECT FOR SHARE", "SELECT * FROM users FOR SHARE"},
		{"SELECT FOR NO KEY UPDATE", "SELECT * FROM users FOR NO KEY UPDATE"},
		{"SELECT FOR KEY SHARE", "SELECT * FROM users FOR KEY SHARE"},
		{"PREPARE", "PREPARE stmt AS SELECT 1"},
		{"DEALLOCATE", "DEALLOCATE stmt"},
		{"EXECUTE", "EXECUTE stmt"},
		{"comment hiding INSERT", "SELECT 1; /* safe */ INSERT INTO users VALUES (1)"},
		{"line comment hiding DELETE", "DELETE FROM users -- just a comment"},
		{"CREATE FUNCTION", "CREATE FUNCTION test() RETURNS void AS $$ BEGIN END $$ LANGUAGE plpgsql"},
		{"REINDEX", "REINDEX TABLE users"},
		{"CLUSTER", "CLUSTER users USING users_pkey"},
		{"DISCARD", "DISCARD ALL"},
		{"LISTEN", "LISTEN my_channel"},
		{"NOTIFY", "NOTIFY my_channel"},
		{"RESET", "RESET work_mem"},
		{"INSERT lowercase", "insert into users (name) values ('test')"},
		{"INSERT mixed case", "Insert Into users (name) Values ('test')"},
		{"DROP with leading whitespace", "  DROP TABLE users"},
		{"ALTER VIEW", "ALTER VIEW myview RENAME TO newview"},
		{"CREATE SCHEMA", "CREATE SCHEMA myschema"},
		{"CREATE TYPE", "CREATE TYPE mood AS ENUM ('happy', 'sad')"},
	}

	for _, tc := range blocked {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.sql)
			if err == nil {
				t.Errorf("expected query to be blocked, but it was allowed\n  SQL: %s", tc.sql)
			}
		})
	}
}

// --- AST Tests ---

func TestCheckAST_EmptyStatement(t *testing.T) {
	err := CheckAST("")
	if err == nil {
		t.Fatal("expected error for empty SQL")
	}
}

func TestCheckAST_MultipleSelectStatements(t *testing.T) {
	err := CheckAST("SELECT 1; SELECT 2")
	if err != nil {
		t.Errorf("multiple SELECT statements should be allowed: %v", err)
	}
}

func TestCheckAST_SelectWithSubquery(t *testing.T) {
	err := CheckAST("SELECT * FROM (SELECT 1 AS x) sub")
	if err != nil {
		t.Errorf("subquery should be allowed: %v", err)
	}
}

func TestCheckAST_ExplainAnalyze(t *testing.T) {
	err := CheckAST("EXPLAIN ANALYZE SELECT * FROM users")
	if err != nil {
		t.Errorf("EXPLAIN ANALYZE SELECT should be allowed: %v", err)
	}
}

func TestCheckAST_ExplainInsert(t *testing.T) {
	err := CheckAST("EXPLAIN INSERT INTO users (name) VALUES ('test')")
	if err == nil {
		t.Fatal("EXPLAIN INSERT should be blocked")
	}
}

func TestCheckAST_SelectIntoBlocked(t *testing.T) {
	err := CheckAST("SELECT * INTO new_table FROM users")
	if err == nil {
		t.Fatal("SELECT INTO should be blocked")
	}
	var fe *ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("expected ForbiddenError, got %T", err)
	}
}

func TestCheckAST_ForUpdateBlocked(t *testing.T) {
	err := CheckAST("SELECT * FROM users FOR UPDATE")
	if err == nil {
		t.Fatal("SELECT FOR UPDATE should be blocked")
	}
}

func TestCheckAST_ForShareBlocked(t *testing.T) {
	err := CheckAST("SELECT * FROM users FOR SHARE")
	if err == nil {
		t.Fatal("SELECT FOR SHARE should be blocked")
	}
}

func TestCheckAST_CTEWithInsertBlocked(t *testing.T) {
	err := CheckAST("WITH ins AS (INSERT INTO users (name) VALUES ('test') RETURNING id) SELECT * FROM ins")
	if err == nil {
		t.Fatal("CTE with INSERT should be blocked")
	}
}

func TestCheckAST_UnionWithSelectIntoBlocked(t *testing.T) {
	err := CheckAST("SELECT 1 UNION SELECT * INTO new_table FROM users")
	if err == nil {
		t.Fatal("UNION with SELECT INTO should be blocked")
	}
}

// --- Validate Tests ---

func TestValidate_ReturnsCorrectErrorType(t *testing.T) {
	err := Validate("DROP TABLE users")
	if err == nil {
		t.Fatal("expected error")
	}
	var fe *ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("expected ForbiddenError, got %T: %v", err, err)
	}
}

func TestForbiddenError_Message(t *testing.T) {
	err := &ForbiddenError{Reason: "test reason"}
	expected := "query blocked: test reason"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestForbiddenError_Implements_error(t *testing.T) {
	var _ error = &ForbiddenError{}
}

func TestValidate_EmptyString(t *testing.T) {
	err := Validate("")
	if err == nil {
		t.Fatal("expected error for empty SQL")
	}
}

func TestValidate_WhitespaceOnly(t *testing.T) {
	err := Validate("   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only SQL")
	}
}
