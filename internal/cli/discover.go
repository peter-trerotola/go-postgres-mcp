package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDiscoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover [database]",
		Short: "Discover or refresh database schemas",
		Long:  "Crawl database schemas and populate the knowledge map cache. If a database name is given, only that database is discovered. Otherwise, all configured databases are discovered.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := connectDB(cmd); err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			if len(args) > 0 {
				dr, err := eng.Discover(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				fmt.Fprintf(w, "Discovered schema for %q (%d tables in knowledge map)\n", args[0], dr.TablesFound)
				return nil
			}

			dr := eng.DiscoverAll(cmd.Context())
			if len(dr.Failures) > 0 {
				for _, f := range dr.Failures {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", f)
				}
			}
			fmt.Fprintf(w, "Discovered %d databases (%d tables in knowledge map)\n", dr.DatabasesDiscovered, dr.TablesFound)
			return nil
		},
	}
}
