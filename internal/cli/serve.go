package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/peter-trerotola/goro-pg/internal/config"
	"github.com/peter-trerotola/goro-pg/internal/server"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start MCP stdio server",
		Long:  "Start the MCP (Model Context Protocol) server over stdio transport for use with Claude Desktop, Claude Code, and other MCP clients.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			app, err := server.New(cfg)
			if err != nil {
				return fmt.Errorf("creating server: %w", err)
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			if err := app.Start(ctx); err != nil {
				app.Shutdown()
				return fmt.Errorf("starting server: %w", err)
			}

			go func() {
				<-ctx.Done()
				log.Println("shutting down...")
				app.Shutdown()
			}()

			log.Println("MCP server started, listening on stdio")
			return app.ServeStdio()
		},
	}
}
