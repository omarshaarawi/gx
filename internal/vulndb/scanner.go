package vulndb

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID          string
	Package     string
	Severity    string
	Description string
	Fixed       string
	Installed   string
	URL         string
}

// ScanResult contains the results of a vulnerability scan
type ScanResult struct {
	Vulnerabilities []*Vulnerability
	TotalScanned    int
	TotalVulns      int
}

// Scanner handles vulnerability scanning
type Scanner struct{}

// NewScanner creates a new vulnerability scanner
func NewScanner() (*Scanner, error) {
	if _, err := exec.LookPath("govulncheck"); err != nil {
		return nil, fmt.Errorf("govulncheck not found. Install it with: go install golang.org/x/vuln/cmd/govulncheck@latest")
	}

	return &Scanner{}, nil
}

// govulncheckMessage represents a JSON message from govulncheck
type govulncheckMessage struct {
	OSV *struct {
		ID       string `json:"id"`
		Summary  string `json:"summary"`
		Details  string `json:"details"`
		Aliases  []string `json:"aliases"`
		DatabaseSpecific *struct {
			Severity string `json:"severity"`
		} `json:"database_specific"`
		Affected []struct {
			Package struct {
				Name      string `json:"name"`
				Ecosystem string `json:"ecosystem"`
			} `json:"package"`
			Ranges []struct {
				Type   string `json:"type"`
				Events []struct {
					Introduced string `json:"introduced"`
					Fixed      string `json:"fixed"`
				} `json:"events"`
			} `json:"ranges"`
		} `json:"affected"`
	} `json:"osv"`
	Finding *struct {
		OSV   string `json:"osv"`
		FixedVersion string `json:"fixed_version"`
	} `json:"finding"`
}

// ScanModule scans a module for vulnerabilities using govulncheck
func (s *Scanner) ScanModule(ctx context.Context, modPath string) (*ScanResult, error) {
	cmd := exec.CommandContext(ctx, "govulncheck", "-json", "./...")
	output, err := cmd.CombinedOutput()

	result := &ScanResult{
		Vulnerabilities: []*Vulnerability{},
	}

	if err != nil {
		// govulncheck exits with non-zero if vulnerabilities are found
		if len(output) == 0 {
			return nil, fmt.Errorf("govulncheck failed: %w", err)
		}
	}

	vulnMap := make(map[string]*Vulnerability)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg govulncheckMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		if msg.OSV != nil {
			osv := msg.OSV

			severity := "UNKNOWN"
			if osv.DatabaseSpecific != nil && osv.DatabaseSpecific.Severity != "" {
				severity = strings.ToUpper(osv.DatabaseSpecific.Severity)
			}

			for _, affected := range osv.Affected {
				pkgName := affected.Package.Name

				fixedVersion := "unknown"
				for _, r := range affected.Ranges {
					for _, event := range r.Events {
						if event.Fixed != "" {
							fixedVersion = event.Fixed
							break
						}
					}
				}

				vuln := &Vulnerability{
					ID:          osv.ID,
					Package:     pkgName,
					Severity:    severity,
					Description: osv.Summary,
					Fixed:       fixedVersion,
					URL:         fmt.Sprintf("https://pkg.go.dev/vuln/%s", osv.ID),
				}

				vulnMap[osv.ID+pkgName] = vuln
			}
		}
	}

	for _, vuln := range vulnMap {
		result.Vulnerabilities = append(result.Vulnerabilities, vuln)
	}

	result.TotalVulns = len(result.Vulnerabilities)
	result.TotalScanned = 1

	return result, nil
}

// FilterBySeverity filters vulnerabilities by severity
func FilterBySeverity(vulns []*Vulnerability, severities []string) []*Vulnerability {
	if len(severities) == 0 {
		return vulns
	}

	severityMap := make(map[string]bool)
	for _, s := range severities {
		severityMap[s] = true
	}

	filtered := []*Vulnerability{}
	for _, v := range vulns {
		if severityMap[v.Severity] {
			filtered = append(filtered, v)
		}
	}

	return filtered
}

