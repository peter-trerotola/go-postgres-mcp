package cli

import (
	"github.com/spf13/cobra"
)

func newSchemasCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schemas [database]",
		Short: "List schemas in a database",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := resolveDB(args)
			if err != nil {
				return err
			}

			schemas, err := eng.ListSchemas(db)
			if err != nil {
				return err
			}

			format := resolveFormat()
			w := cmd.OutOrStdout()

			headers := []string{"schema_name"}
			rows := make([][]string, len(schemas))
			for i, s := range schemas {
				rows[i] = []string{s.SchemaName}
			}
			return formatOutput(w, format, headers, rows, schemas)
		},
	}
}
