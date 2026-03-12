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
// FROM clauses, JOINs, subqueries (WHERE, HAVING, SELECT list), CTEs, and
// UNION branches.
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
			collectFromSelect(s.SelectStmt, seen, &refs, nil)
		case *pg_query.Node_ExplainStmt:
			if s.ExplainStmt.Query != nil {
				if inner, ok := s.ExplainStmt.Query.Node.(*pg_query.Node_SelectStmt); ok {
					collectFromSelect(inner.SelectStmt, seen, &refs, nil)
				}
			}
		}
	}

	return refs
}

// collectFromSelect walks a SelectStmt and collects table references.
// When cteNames is non-nil (filter mode), CTE names are recorded and
// unqualified references matching CTE names are skipped.
func collectFromSelect(sel *pg_query.SelectStmt, seen map[TableRef]struct{}, refs *[]TableRef, cteNames map[string]struct{}) {
	if sel == nil {
		return
	}

	// CTEs — record names before processing bodies so inner refs can be skipped
	if sel.WithClause != nil {
		for _, cte := range sel.WithClause.Ctes {
			if cte == nil {
				continue
			}
			if cteExpr, ok := cte.Node.(*pg_query.Node_CommonTableExpr); ok && cteExpr.CommonTableExpr != nil {
				if cteNames != nil {
					cteNames[cteExpr.CommonTableExpr.Ctename] = struct{}{}
				}
				if inner := cteExpr.CommonTableExpr.Ctequery; inner != nil {
					if innerSel, ok := inner.Node.(*pg_query.Node_SelectStmt); ok {
						collectFromSelect(innerSel.SelectStmt, seen, refs, cteNames)
					}
				}
			}
		}
	}

	// FROM clause
	for _, node := range sel.FromClause {
		collectFromNode(node, seen, refs, cteNames)
	}

	// WHERE clause subqueries
	walkNodeForSubqueries(sel.WhereClause, seen, refs, cteNames)

	// HAVING clause subqueries
	walkNodeForSubqueries(sel.HavingClause, seen, refs, cteNames)

	// SELECT list subqueries
	for _, target := range sel.TargetList {
		walkNodeForSubqueries(target, seen, refs, cteNames)
	}

	// UNION/INTERSECT/EXCEPT branches
	collectFromSelect(sel.Larg, seen, refs, cteNames)
	collectFromSelect(sel.Rarg, seen, refs, cteNames)
}

// collectFromNode extracts table references from a FROM-clause node.
// When cteNames is non-nil, unqualified references matching CTE names
// are skipped (they are query-local aliases, not real tables).
func collectFromNode(node *pg_query.Node, seen map[TableRef]struct{}, refs *[]TableRef, cteNames map[string]struct{}) {
	if node == nil {
		return
	}

	switch n := node.Node.(type) {
	case *pg_query.Node_RangeVar:
		rv := n.RangeVar
		if rv == nil {
			return
		}
		// Skip unqualified CTE names — only when Schemaname is empty (unqualified).
		// Explicitly qualified refs (e.g. public.users) are always checked, even if
		// a CTE with the same name exists, to prevent filter bypass.
		if cteNames != nil && rv.Schemaname == "" {
			if _, isCTE := cteNames[rv.Relname]; isCTE {
				return
			}
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
			collectFromNode(n.JoinExpr.Larg, seen, refs, cteNames)
			collectFromNode(n.JoinExpr.Rarg, seen, refs, cteNames)
		}

	case *pg_query.Node_RangeSubselect:
		if n.RangeSubselect != nil && n.RangeSubselect.Subquery != nil {
			if sub, ok := n.RangeSubselect.Subquery.Node.(*pg_query.Node_SelectStmt); ok {
				collectFromSelect(sub.SelectStmt, seen, refs, cteNames)
			}
		}
	}
}

// walkNodeForSubqueries recursively walks an expression node tree looking for
// SubLink nodes (subqueries in WHERE, HAVING, SELECT list, etc.) and extracts
// table references from their contained SELECT statements.
func walkNodeForSubqueries(node *pg_query.Node, seen map[TableRef]struct{}, refs *[]TableRef, cteNames map[string]struct{}) {
	if node == nil {
		return
	}

	switch n := node.Node.(type) {
	case *pg_query.Node_SubLink:
		if n.SubLink != nil && n.SubLink.Subselect != nil {
			if sel, ok := n.SubLink.Subselect.Node.(*pg_query.Node_SelectStmt); ok {
				collectFromSelect(sel.SelectStmt, seen, refs, cteNames)
			}
		}
		// Also walk the test expression (left side of IN, ANY, etc.)
		if n.SubLink != nil {
			walkNodeForSubqueries(n.SubLink.Testexpr, seen, refs, cteNames)
		}

	case *pg_query.Node_BoolExpr:
		if n.BoolExpr != nil {
			for _, arg := range n.BoolExpr.Args {
				walkNodeForSubqueries(arg, seen, refs, cteNames)
			}
		}

	case *pg_query.Node_AExpr:
		if n.AExpr != nil {
			walkNodeForSubqueries(n.AExpr.Lexpr, seen, refs, cteNames)
			walkNodeForSubqueries(n.AExpr.Rexpr, seen, refs, cteNames)
		}

	case *pg_query.Node_FuncCall:
		if n.FuncCall != nil {
			for _, arg := range n.FuncCall.Args {
				walkNodeForSubqueries(arg, seen, refs, cteNames)
			}
		}

	case *pg_query.Node_CaseExpr:
		if n.CaseExpr != nil {
			walkNodeForSubqueries(n.CaseExpr.Arg, seen, refs, cteNames)
			for _, w := range n.CaseExpr.Args {
				walkNodeForSubqueries(w, seen, refs, cteNames)
			}
			walkNodeForSubqueries(n.CaseExpr.Defresult, seen, refs, cteNames)
		}

	case *pg_query.Node_CaseWhen:
		if n.CaseWhen != nil {
			walkNodeForSubqueries(n.CaseWhen.Expr, seen, refs, cteNames)
			walkNodeForSubqueries(n.CaseWhen.Result, seen, refs, cteNames)
		}

	case *pg_query.Node_CoalesceExpr:
		if n.CoalesceExpr != nil {
			for _, arg := range n.CoalesceExpr.Args {
				walkNodeForSubqueries(arg, seen, refs, cteNames)
			}
		}

	case *pg_query.Node_ResTarget:
		if n.ResTarget != nil {
			walkNodeForSubqueries(n.ResTarget.Val, seen, refs, cteNames)
		}

	case *pg_query.Node_TypeCast:
		if n.TypeCast != nil {
			walkNodeForSubqueries(n.TypeCast.Arg, seen, refs, cteNames)
		}

	case *pg_query.Node_NullTest:
		if n.NullTest != nil {
			walkNodeForSubqueries(n.NullTest.Arg, seen, refs, cteNames)
		}

	case *pg_query.Node_BooleanTest:
		if n.BooleanTest != nil {
			walkNodeForSubqueries(n.BooleanTest.Arg, seen, refs, cteNames)
		}

	case *pg_query.Node_RowExpr:
		if n.RowExpr != nil {
			for _, arg := range n.RowExpr.Args {
				walkNodeForSubqueries(arg, seen, refs, cteNames)
			}
		}
	}
}

// CheckTableFilter parses the SQL, extracts all table references, and validates
// each against the provided filter function. CTE names are collected during
// traversal and unqualified references matching CTE names are skipped (they are
// query-local aliases, not real tables). Explicitly schema-qualified references
// are always checked, even if a CTE with the same name exists.
// Returns a ForbiddenError if any referenced table is not allowed by the filter.
func CheckTableFilter(sql string, allowed func(schema, table string) bool) error {
	tree, err := pg_query.Parse(sql)
	if err != nil {
		return nil // parse errors are handled by Validate()
	}

	var refs []TableRef
	seen := make(map[TableRef]struct{})
	cteNames := make(map[string]struct{})

	for _, stmt := range tree.Stmts {
		if stmt.Stmt == nil {
			continue
		}
		switch s := stmt.Stmt.Node.(type) {
		case *pg_query.Node_SelectStmt:
			collectFromSelect(s.SelectStmt, seen, &refs, cteNames)
		case *pg_query.Node_ExplainStmt:
			if s.ExplainStmt.Query != nil {
				if inner, ok := s.ExplainStmt.Query.Node.(*pg_query.Node_SelectStmt); ok {
					collectFromSelect(inner.SelectStmt, seen, &refs, cteNames)
				}
			}
		}
	}

	for _, ref := range refs {
		if !allowed(ref.Schema, ref.Table) {
			return &ForbiddenError{Reason: fmt.Sprintf("query references restricted table %q", ref.Schema+"."+ref.Table)}
		}
	}

	return nil
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
	if err := checkCTEs(sel.WithClause); err != nil {
		return err
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

// checkCTEs validates that all CTEs in a WITH clause contain only SELECT statements.
func checkCTEs(wc *pg_query.WithClause) error {
	if wc == nil {
		return nil
	}
	for _, cte := range wc.Ctes {
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
	return nil
}
