package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

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

			var sqlStr string
			if len(args) == 0 || args[0] == "-" {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				sqlStr = strings.TrimSpace(string(data))
			} else {
				sqlStr = args[0]
			}

			if sqlStr == "" {
				return fmt.Errorf("SQL query required")
			}

			result, err := eng.Query(cmd.Context(), db, sqlStr)
			if err != nil {
				return err
			}

			format := resolveFormat()
			w := cmd.OutOrStdout()

			if format == FormatJSON {
				return writeJSON(w, result)
			}

			// Build tabular output from dynamic columns
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
		},
	}
}
