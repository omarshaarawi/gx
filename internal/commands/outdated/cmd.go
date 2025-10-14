package outdated

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagDirectOnly bool
	flagMajorOnly  bool
)

// NewCommand creates the outdated command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outdated",
		Short: "Show outdated dependencies",
		Long: `Show outdated dependencies in a table format.

Examples:
  # Show all outdated packages
  gx outdated

  # Show only direct dependencies
  gx outdated --direct-only

  # Show only major version updates
  gx outdated --major-only`,
		RunE: runOutdated,
	}

	cmd.Flags().BoolVar(&flagDirectOnly, "direct-only", false, "Show only direct dependencies")
	cmd.Flags().BoolVar(&flagMajorOnly, "major-only", false, "Show only major version updates")

	return cmd
}

func runOutdated(cmd *cobra.Command, args []string) error {
	modPath := "go.mod"
	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found in current directory")
	}

	opts := Options{
		DirectOnly: flagDirectOnly,
		MajorOnly:  flagMajorOnly,
		ModPath:    modPath,
	}

	return Run(opts)
}
