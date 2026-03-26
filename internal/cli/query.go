package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/peter-trerotola/goro-pg/internal/postgres"
	"github.com/spf13/cobra"
)

func newQueryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "query <sql>",
		Short: "Execute a read-only SQL query",
		Long:  "Execute a read-only SELECT query against a PostgreSQL database. Use '-' as the SQL argument to read from stdin.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := resolveDB(nil)
			if err != nil {
				return err
			}
			if err := connectDB(cmd); err != nil {
				return err
			}

			sqlStr, err := resolveSQL(args)
			if err != nil {
				return err
			}

			result, err := eng.Query(cmd.Context(), db, sqlStr)
			if err != nil {
				return err
			}

			return renderQueryResult(cmd, result)
		},
	}
}

// resolveSQL extracts the SQL string from args or stdin.
func resolveSQL(args []string) (string, error) {
	if len(args) > 0 && args[0] != "-" {
		return args[0], nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	sql := strings.TrimSpace(string(data))
	if sql == "" {
		return "", fmt.Errorf("SQL query required")
	}
	return sql, nil
}

// renderQueryResult outputs a QueryResult in the resolved format.
func renderQueryResult(cmd *cobra.Command, result *postgres.QueryResult) error {
	format := resolveFormat()
	w := cmd.OutOrStdout()

	if format == FormatJSON {
		return writeJSON(w, result)
	}

	headers, rows := queryResultToRows(result)
	if err := formatOutput(w, format, headers, rows, result); err != nil {
		return err
	}

	if format == FormatTable {
		if result.Truncated {
			fmt.Fprintf(w, "(%d rows, truncated)\n", result.Count)
		} else {
			fmt.Fprintf(w, "(%d rows)\n", result.Count)
		}
	}
	return nil
}

// queryResultToRows converts dynamic query result rows into string slices.
func queryResultToRows(result *postgres.QueryResult) ([]string, [][]string) {
	headers := result.Columns
	rows := make([][]string, 0, len(result.Rows))
	for _, raw := range result.Rows {
		var rowMap map[string]any
		if err := json.Unmarshal(raw, &rowMap); err != nil {
			continue
		}
		row := make([]string, len(headers))
		for i, col := range headers {
			if v, ok := rowMap[col]; ok {
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		rows = append(rows, row)
	}
	return headers, rows
}
