package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/petros/go-postgres-mcp/internal/guard"
	"github.com/petros/go-postgres-mcp/internal/knowledgemap"
)

const maxRows = 1000

// QueryResult holds the result of a read-only query.
type QueryResult struct {
	Columns       []string                              `json:"columns"`
	Rows          []json.RawMessage                     `json:"rows"`
	Count         int                                   `json:"count"`
	Truncated     bool                                  `json:"truncated"`
	SchemaContext map[string][]knowledgemap.ColumnSummary `json:"schema_context,omitempty"`
}

// ReadOnlyQuery executes a SQL query with full read-only enforcement:
//   - Tier 1: guard.Validate (AST parser via pg_query_go)
//   - Tier 2: connection-level default_transaction_read_only=on (set in pool.go)
//   - Tier 3: transaction-level BEGIN READ ONLY
func ReadOnlyQuery(ctx context.Context, pm *PoolManager, dbName, sql string) (*QueryResult, error) {
	if err := guard.Validate(sql); err != nil {
		return nil, err
	}

	pool, err := pm.Get(dbName)
	if err != nil {
		return nil, err
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("beginning read-only transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	columns := make([]string, 0)
	for _, fd := range rows.FieldDescriptions() {
		columns = append(columns, fd.Name)
	}

	var resultRows []json.RawMessage
	count := 0
	truncated := false
	for rows.Next() {
		if count >= maxRows {
			truncated = true
			rows.Close()
			break
		}
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("reading row: %w", err)
		}

		rowMap := make(map[string]interface{})
		for i, col := range columns {
			rowMap[col] = values[i]
		}

		rowJSON, err := json.Marshal(rowMap)
		if err != nil {
			return nil, fmt.Errorf("marshaling row: %w", err)
		}
		resultRows = append(resultRows, rowJSON)
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing read-only transaction: %w", err)
	}

	return &QueryResult{
		Columns:   columns,
		Rows:      resultRows,
		Count:     count,
		Truncated: truncated,
	}, nil
}
