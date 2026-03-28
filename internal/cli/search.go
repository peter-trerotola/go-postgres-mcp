package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across schema metadata",
		Long:  "Search across all schema metadata in the knowledge map using FTS5 keywords.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := eng.SearchSchema(args[0])
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No results found.")
				return nil
			}

			format := resolveFormat()
			w := cmd.OutOrStdout()

			headers := []string{"database", "schema", "type", "name", "detail"}
			rows := make([][]string, len(results))
			for i, r := range results {
				rows[i] = []string{r.DatabaseName, r.SchemaName, r.ObjectType, r.ObjectName, r.Detail}
			}
			return formatOutput(w, format, headers, rows, results)
		},
	}
}
