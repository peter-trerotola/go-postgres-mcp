package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newViewsCmd() *cobra.Command {
	var schema string

	cmd := &cobra.Command{
		Use:   "views [database]",
		Short: "List views in a schema",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := resolveDB(args)
			if err != nil {
				return err
			}

			views, err := eng.ListViews(db, schema)
			if err != nil {
				return err
			}

			if len(views) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "No views found. Run 'goro-pg discover' first to crawl database schemas.\n")
				return nil
			}

			format := resolveFormat()
			w := cmd.OutOrStdout()

			headers := []string{"schema", "view", "definition"}
			rows := make([][]string, len(views))
			for i, v := range views {
				rows[i] = []string{v.SchemaName, v.ViewName, nullStr(v.Definition)}
			}
			return formatOutput(w, format, headers, rows, views)
		},
	}

	cmd.Flags().StringVarP(&schema, "schema", "s", "public", "schema name")
	return cmd
}
