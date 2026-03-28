package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTablesCmd() *cobra.Command {
	var schema string

	cmd := &cobra.Command{
		Use:   "tables [database]",
		Short: "List tables in a schema",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := resolveDB(args)
			if err != nil {
				return err
			}

			tables, err := eng.ListTables(db, schema)
			if err != nil {
				return err
			}

			if len(tables) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "No tables found. Run 'goro-pg discover' first to crawl database schemas.\n")
				return nil
			}

			format := resolveFormat()
			w := cmd.OutOrStdout()

			headers := []string{"schema", "table", "type", "rows", "size"}
			rows := make([][]string, len(tables))
			for i, t := range tables {
				rows[i] = []string{
					t.SchemaName,
					t.TableName,
					t.TableType,
					fmt.Sprintf("%d", t.RowEstimate),
					fmt.Sprintf("%d", t.SizeBytes),
				}
			}
			return formatOutput(w, format, headers, rows, tables)
		},
	}

	cmd.Flags().StringVarP(&schema, "schema", "s", "public", "schema name")
	return cmd
}
