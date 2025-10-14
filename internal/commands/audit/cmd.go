package audit

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	flagSeverity string
	flagJSON     bool
)

// NewCommand creates the audit command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Scan dependencies for known vulnerabilities",
		Long: `Scan dependencies for known vulnerabilities using the Go vulnerability database.

Examples:
  # Scan all dependencies
  gx audit

  # Filter by severity (critical, high, medium, low)
  gx audit --severity=high,critical

  # JSON output for scripting
  gx audit --json

  # Save report to file
  gx audit --json > report.json`,
		RunE: runAudit,
	}

	cmd.Flags().StringVar(&flagSeverity, "severity", "", "Filter by severity (comma-separated: critical,high,medium,low)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "Output results as JSON")

	return cmd
}

func runAudit(cmd *cobra.Command, args []string) error {
	modPath := "go.mod"
	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found in current directory")
	}

	var severities []string
	if flagSeverity != "" {
		severities = strings.Split(flagSeverity, ",")
		for i, s := range severities {
			severities[i] = strings.ToUpper(strings.TrimSpace(s))
		}
	}

	opts := Options{
		Severity: severities,
		JSON:     flagJSON,
		ModPath:  modPath,
	}

	return Run(opts)
}

