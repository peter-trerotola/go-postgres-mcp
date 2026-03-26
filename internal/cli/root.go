package cli

import (
	"fmt"
	"os"

	"github.com/peter-trerotola/goro-pg/internal/config"
	"github.com/peter-trerotola/goro-pg/internal/engine"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	formatFlag string
	dbFlag     string

	eng *engine.Engine

	// version is set via ldflags at build time.
	version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "goro-pg",
	Short: "Read-only PostgreSQL explorer with schema intelligence",
	Long:  "goro-pg provides read-only access to PostgreSQL databases with schema discovery, full-text search, and query execution.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip engine init for serve and version commands
		if cmd.Name() == "serve" || cmd.Name() == "version" {
			return nil
		}
		return initEngine(cmd)
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if eng != nil {
			eng.Shutdown()
		}
	},
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config.yaml", "config file path")
	rootCmd.PersistentFlags().StringVarP(&formatFlag, "format", "f", "", "output format: table, json, csv, plain (default: auto-detect)")
	rootCmd.PersistentFlags().StringVarP(&dbFlag, "database", "d", "", "default database name")

	rootCmd.AddCommand(newQueryCmd())
	rootCmd.AddCommand(newDiscoverCmd())
	rootCmd.AddCommand(newDatabasesCmd())
	rootCmd.AddCommand(newSchemasCmd())
	rootCmd.AddCommand(newTablesCmd())
	rootCmd.AddCommand(newDescribeCmd())
	rootCmd.AddCommand(newViewsCmd())
	rootCmd.AddCommand(newFunctionsCmd())
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newVersionCmd())
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// initEngine loads config and creates the engine. Commands that need
// live database connections call connectDB separately.
func initEngine(cmd *cobra.Command) error {
	// Check env for config path
	if cfgFile == "config.yaml" {
		if envCfg := os.Getenv("GORO_PG_CONFIG"); envCfg != "" {
			cfgFile = envCfg
		}
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	eng, err = engine.New(cfg)
	if err != nil {
		return fmt.Errorf("initializing engine: %w", err)
	}

	// Check env for default database
	if dbFlag == "" {
		if envDB := os.Getenv("GORO_PG_DATABASE"); envDB != "" {
			dbFlag = envDB
		}
	}

	return nil
}

// connectDB connects the engine to all configured databases.
func connectDB(cmd *cobra.Command) error {
	return eng.Connect(cmd.Context())
}

// resolveDB returns the database name from the flag, positional arg, or error.
func resolveDB(args []string) (string, error) {
	if len(args) > 0 && args[0] != "" {
		return args[0], nil
	}
	if dbFlag != "" {
		return dbFlag, nil
	}
	return "", fmt.Errorf("database name required: use -d flag or pass as argument")
}

// resolveFormat returns the output format from the flag or auto-detection.
func resolveFormat() string {
	if formatFlag != "" {
		return formatFlag
	}
	return defaultFormat()
}
