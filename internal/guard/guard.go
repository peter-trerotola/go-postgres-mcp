package guard

import "fmt"

// ForbiddenError indicates a SQL statement was rejected by the guard.
type ForbiddenError struct {
	Reason string
}

func (e *ForbiddenError) Error() string {
	return fmt.Sprintf("query blocked: %s", e.Reason)
}

// Validate checks SQL through read-only enforcement using PostgreSQL's
// actual parser (pg_query_go) to structurally verify only SELECT statements
// are present.
func Validate(sql string) error {
	return CheckAST(sql)
}
