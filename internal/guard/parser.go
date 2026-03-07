package guard

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v5"
)

// TableRef represents a table referenced in a SQL statement.
type TableRef struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

// ExtractTableRefs parses SQL and returns all table references found in
// FROM clauses, JOINs, subqueries, CTEs, and UNION branches.
func ExtractTableRefs(sql string) []TableRef {
	tree, err := pg_query.Parse(sql)
	if err != nil {
		return nil
	}

	seen := make(map[TableRef]struct{})
	var refs []TableRef

	for _, stmt := range tree.Stmts {
		if stmt.Stmt == nil {
			continue
		}
		switch s := stmt.Stmt.Node.(type) {
		case *pg_query.Node_SelectStmt:
			collectFromSelect(s.SelectStmt, seen, &refs)
		case *pg_query.Node_ExplainStmt:
			if s.ExplainStmt.Query != nil {
				if inner, ok := s.ExplainStmt.Query.Node.(*pg_query.Node_SelectStmt); ok {
					collectFromSelect(inner.SelectStmt, seen, &refs)
				}
			}
		}
	}

	return refs
}

func collectFromSelect(sel *pg_query.SelectStmt, seen map[TableRef]struct{}, refs *[]TableRef) {
	if sel == nil {
		return
	}

	// CTEs
	if sel.WithClause != nil {
		for _, cte := range sel.WithClause.Ctes {
			if cte == nil {
				continue
			}
			if cteExpr, ok := cte.Node.(*pg_query.Node_CommonTableExpr); ok && cteExpr.CommonTableExpr != nil {
				if inner := cteExpr.CommonTableExpr.Ctequery; inner != nil {
					if innerSel, ok := inner.Node.(*pg_query.Node_SelectStmt); ok {
						collectFromSelect(innerSel.SelectStmt, seen, refs)
					}
				}
			}
		}
	}

	// FROM clause
	for _, node := range sel.FromClause {
		collectFromNode(node, seen, refs)
	}

	// WHERE subqueries are handled via target list / where clause nodes
	// but we only extract FROM-clause table refs per the plan

	// UNION/INTERSECT/EXCEPT branches
	collectFromSelect(sel.Larg, seen, refs)
	collectFromSelect(sel.Rarg, seen, refs)
}

func collectFromNode(node *pg_query.Node, seen map[TableRef]struct{}, refs *[]TableRef) {
	if node == nil {
		return
	}

	switch n := node.Node.(type) {
	case *pg_query.Node_RangeVar:
		rv := n.RangeVar
		if rv == nil {
			return
		}
		schema := rv.Schemaname
		if schema == "" {
			// Default to "public" — matches PostgreSQL's default search_path.
			// For non-public default schemas, users should schema-qualify their queries.
			schema = "public"
		}
		ref := TableRef{Schema: schema, Table: rv.Relname}
		if _, exists := seen[ref]; !exists {
			seen[ref] = struct{}{}
			*refs = append(*refs, ref)
		}

	case *pg_query.Node_JoinExpr:
		if n.JoinExpr != nil {
			collectFromNode(n.JoinExpr.Larg, seen, refs)
			collectFromNode(n.JoinExpr.Rarg, seen, refs)
		}

	case *pg_query.Node_RangeSubselect:
		if n.RangeSubselect != nil && n.RangeSubselect.Subquery != nil {
			if sub, ok := n.RangeSubselect.Subquery.Node.(*pg_query.Node_SelectStmt); ok {
				collectFromSelect(sub.SelectStmt, seen, refs)
			}
		}
	}
}

// CheckAST parses the SQL using PostgreSQL's actual parser and validates
// that it contains only safe read-only statements. This is Tier 1 of
// read-only enforcement.
func CheckAST(sql string) error {
	tree, err := pg_query.Parse(sql)
	if err != nil {
		return &ForbiddenError{Reason: fmt.Sprintf("SQL parse error: %v", err)}
	}

	if len(tree.Stmts) == 0 {
		return &ForbiddenError{Reason: "empty SQL statement"}
	}

	for _, stmt := range tree.Stmts {
		node := stmt.Stmt
		if node == nil {
			return &ForbiddenError{Reason: "nil statement node"}
		}

		switch s := node.Node.(type) {
		case *pg_query.Node_SelectStmt:
			if err := checkSelectStmt(s.SelectStmt); err != nil {
				return err
			}
		case *pg_query.Node_ExplainStmt:
			// EXPLAIN is allowed, but validate the inner statement
			inner := s.ExplainStmt.Query
			if inner == nil {
				return &ForbiddenError{Reason: "EXPLAIN with nil inner query"}
			}
			switch innerS := inner.Node.(type) {
			case *pg_query.Node_SelectStmt:
				if err := checkSelectStmt(innerS.SelectStmt); err != nil {
					return err
				}
			default:
				return &ForbiddenError{Reason: "EXPLAIN is only allowed for SELECT statements"}
			}
		default:
			return &ForbiddenError{Reason: fmt.Sprintf("only SELECT statements are allowed, got %T", node.Node)}
		}
	}

	return nil
}

// checkSelectStmt validates a SELECT statement to ensure it doesn't
// contain mutations (SELECT INTO, locking clauses, CTEs with mutations).
func checkSelectStmt(sel *pg_query.SelectStmt) error {
	if sel == nil {
		return &ForbiddenError{Reason: "nil SELECT statement"}
	}

	// Reject SELECT INTO (creates a new table)
	if sel.IntoClause != nil {
		return &ForbiddenError{Reason: "SELECT INTO is not allowed"}
	}

	// Reject FOR UPDATE / FOR SHARE locking clauses
	if len(sel.LockingClause) > 0 {
		return &ForbiddenError{Reason: "SELECT with locking clause (FOR UPDATE/SHARE) is not allowed"}
	}

	// Check CTEs (WITH clauses) for mutations
	if sel.WithClause != nil {
		for _, cte := range sel.WithClause.Ctes {
			if cte == nil {
				continue
			}
			cteExpr, ok := cte.Node.(*pg_query.Node_CommonTableExpr)
			if !ok || cteExpr.CommonTableExpr == nil {
				continue
			}
			innerQuery := cteExpr.CommonTableExpr.Ctequery
			if innerQuery == nil {
				continue
			}
			switch inner := innerQuery.Node.(type) {
			case *pg_query.Node_SelectStmt:
				if err := checkSelectStmt(inner.SelectStmt); err != nil {
					return err
				}
			default:
				return &ForbiddenError{Reason: "CTE contains a non-SELECT statement"}
			}
		}
	}

	// Recursively check subqueries in set operations (UNION, INTERSECT, EXCEPT)
	if sel.Larg != nil {
		if err := checkSelectStmt(sel.Larg); err != nil {
			return err
		}
	}
	if sel.Rarg != nil {
		if err := checkSelectStmt(sel.Rarg); err != nil {
			return err
		}
	}

	return nil
}
