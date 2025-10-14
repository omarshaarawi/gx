package update

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagInteractive bool
	flagDryRun      bool
	flagAll         bool
	flagMajor       bool
	flagVendor      bool
)

// NewCommand creates the update command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update Go module dependencies",
		Long: `Update Go module dependencies interactively or automatically.

Examples:
  # Interactive mode (choose which packages to update)
  gx update -i
  gx update --interactive

  # Update all outdated dependencies
  gx update --all

  # Dry run (see what would be updated)
  gx update -i --dry-run

  # Include major version updates
  gx update -i --major`,
		RunE: runUpdate,
	}

	cmd.Flags().BoolVarP(&flagInteractive, "interactive", "i", false, "Interactive mode with TUI")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Show what would be updated without making changes")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Update all outdated dependencies")
	cmd.Flags().BoolVar(&flagMajor, "major", false, "Include major version updates")
	cmd.Flags().BoolVar(&flagVendor, "vendor", false, "Run 'go mod vendor' after tidy")

	return cmd
}

func runUpdate(cmd *cobra.Command, args []string) error {
	modPath := "go.mod"
	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found in current directory")
	}

	opts := Options{
		Interactive: flagInteractive,
		DryRun:      flagDryRun,
		All:         flagAll,
		Major:       flagMajor,
		Vendor:      flagVendor,
		ModPath:     modPath,
	}

	return Run(cmd.Context(), opts)
}

