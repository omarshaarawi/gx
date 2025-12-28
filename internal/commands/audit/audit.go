package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/omarshaarawi/gx/internal/ui"
	"github.com/omarshaarawi/gx/internal/vulndb"
)

// Options configures the audit command
type Options struct {
	Severity []string
	JSON     bool
	ModPath  string
}

// Run executes the audit command
func Run(ctx context.Context, opts Options) error {

	scanner, err := vulndb.NewScanner()
	if err != nil {
		return fmt.Errorf("creating scanner: %w", err)
	}

	result, err := scanModuleWithSpinner(ctx, scanner, opts.ModPath)
	if err != nil {
		return fmt.Errorf("scanning module: %w", err)
	}

	vulns := result.Vulnerabilities
	if len(opts.Severity) > 0 {
		vulns = vulndb.FilterBySeverity(vulns, opts.Severity)
	}

	if opts.JSON {
		return outputJSON(vulns, result)
	}

	return outputTable(vulns, result)
}

func outputJSON(vulns []*vulndb.Vulnerability, result *vulndb.ScanResult) error {
	output := map[string]interface{}{
		"total_scanned":      result.TotalScanned,
		"total_vulnerabilities": len(vulns),
		"vulnerabilities":    vulns,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func outputTable(vulns []*vulndb.Vulnerability, result *vulndb.ScanResult) error {
	if result.TotalScanned > 0 {
		fmt.Printf("\nScanned %d packages\n\n", result.TotalScanned)
	} else {
		fmt.Println()
	}

	if len(vulns) == 0 {
		fmt.Println("✓ No vulnerabilities found!")
		return nil
	}

	bySeverity := make(map[string][]*vulndb.Vulnerability)
	for _, v := range vulns {
		severity := strings.ToUpper(v.Severity)
		if severity == "" {
			severity = "UNKNOWN"
		}
		bySeverity[severity] = append(bySeverity[severity], v)
	}

	severities := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"}

	for _, sev := range severities {
		sevVulns, exists := bySeverity[sev]
		if !exists || len(sevVulns) == 0 {
			continue
		}

		style := ui.SeverityStyle(sev)
		fmt.Printf("\n%s (%d)\n", style.Render(sev), len(sevVulns))
		fmt.Println(strings.Repeat("─", 80))

		for _, v := range sevVulns {
			fmt.Printf("\n%s - %s\n", style.Render(v.ID), v.Package)
			fmt.Printf("  Installed: %s\n", v.Installed)
			if v.Fixed != "unknown" {
				fmt.Printf("  Fixed:     %s\n", v.Fixed)
			}
			if v.Description != "" {
				fmt.Printf("  %s\n", v.Description)
			}
			fmt.Printf("  Details:   %s\n", v.URL)
		}
	}

	fmt.Printf("\n")
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("\nFound %d vulnerabilities:\n", len(vulns))

	for _, sev := range severities {
		if count, exists := bySeverity[sev]; exists && len(count) > 0 {
			style := ui.SeverityStyle(sev)
			fmt.Printf("  %s: %d\n", style.Render(sev), len(count))
		}
	}

	fmt.Println("\nRun 'gx update -i' to update vulnerable packages")

	return nil
}

