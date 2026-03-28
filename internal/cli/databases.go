package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDatabasesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "databases",
		Aliases: []string{"dbs"},
		Short:   "List configured databases",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbs, configDBs, err := eng.ListDatabases()
			if err != nil {
				return err
			}

			format := resolveFormat()
			w := cmd.OutOrStdout()

			if configDBs != nil {
				headers := []string{"name", "host", "database", "status"}
				rows := make([][]string, len(configDBs))
				for i, db := range configDBs {
					rows[i] = []string{db.Name, db.Host, db.Database, db.Status}
				}
				return formatOutput(w, format, headers, rows, configDBs)
			}

			headers := []string{"name", "host", "port", "database", "discovered_at"}
			rows := make([][]string, len(dbs))
			for i, db := range dbs {
				rows[i] = []string{
					db.Name,
					db.Host,
					fmt.Sprintf("%d", db.Port),
					db.Database,
					db.DiscoveredAt,
				}
			}
			return formatOutput(w, format, headers, rows, dbs)
		},
	}
	return cmd
}
