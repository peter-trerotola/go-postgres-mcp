package main

import (
	"os"

	"github.com/peter-trerotola/goro-pg/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
