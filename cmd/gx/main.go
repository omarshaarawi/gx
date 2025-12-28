package main

import (
	"fmt"
	"os"

	"github.com/omarshaarawi/gx/internal/commands/audit"
	"github.com/omarshaarawi/gx/internal/commands/outdated"
	"github.com/omarshaarawi/gx/internal/commands/update"
	"github.com/omarshaarawi/gx/internal/ui"
	"github.com/spf13/cobra"
)

var (
	version     = "dev"
	flagVerbose bool
	flagQuiet   bool
)

var rootCmd = &cobra.Command{
	Use:     "gx",
	Short:   "My personal tooling for Go",
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if flagQuiet {
			ui.SetVerbosity(ui.VerbosityQuiet)
		} else if flagVerbose {
			ui.SetVerbosity(ui.VerbosityVerbose)
		}
	},
}

func init() {
	rootCmd.SetVersionTemplate(`{{.Version}}`)
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.AddCommand(outdated.NewCommand())
	rootCmd.AddCommand(audit.NewCommand())
	rootCmd.AddCommand(update.NewCommand())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
