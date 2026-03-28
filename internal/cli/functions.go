package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newFunctionsCmd() *cobra.Command {
	var schema string

	cmd := &cobra.Command{
		Use:   "functions [database]",
		Short: "List functions in a schema",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := resolveDB(args)
			if err != nil {
				return err
			}

			functions, err := eng.ListFunctions(db, schema)
			if err != nil {
				return err
			}

			if len(functions) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "No functions found. Run 'goro-pg discover' first to crawl database schemas.\n")
				return nil
			}

			format := resolveFormat()
			w := cmd.OutOrStdout()

			headers := []string{"schema", "function", "return_type", "arguments", "language"}
			rows := make([][]string, len(functions))
			for i, f := range functions {
				rows[i] = []string{
					f.SchemaName,
					f.FunctionName,
					nullStr(f.ResultType),
					nullStr(f.ArgTypes),
					nullStr(f.Language),
				}
			}
			return formatOutput(w, format, headers, rows, functions)
		},
	}

	cmd.Flags().StringVarP(&schema, "schema", "s", "public", "schema name")
	return cmd
}
