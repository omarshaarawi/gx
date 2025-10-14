package main

import (
	"fmt"
	"os"

	"github.com/omarshaarawi/gx/internal/commands/audit"
	"github.com/omarshaarawi/gx/internal/commands/outdated"
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "gx",
	Short:   "My personal tooling for Go",
	Version: version,
}

func init() {
	rootCmd.SetVersionTemplate(`{{.Version}}`)
	rootCmd.AddCommand(outdated.NewCommand())
	rootCmd.AddCommand(audit.NewCommand())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
