package vulndb

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewScanner(t *testing.T) {
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	tests := []struct {
		name        string
		setupPath   func(t *testing.T) string
		wantErr     bool
		errContains string
	}{
		{
			name: "govulncheck found in PATH",
			setupPath: func(t *testing.T) string {
				return originalPath
			},
			wantErr: false,
		},
		{
			name: "govulncheck not in PATH",
			setupPath: func(t *testing.T) string {
				return ""
			},
			wantErr:     true,
			errContains: "govulncheck not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newPath := tt.setupPath(t)
			os.Setenv("PATH", newPath)

			scanner, err := NewScanner()

			if tt.wantErr {
				if err == nil {
					t.Error("NewScanner() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("NewScanner() error = %v, should contain %q", err, tt.errContains)
				}
				if scanner != nil {
					t.Error("NewScanner() should return nil scanner on error")
				}
			} else {
				if err != nil {
					t.Errorf("NewScanner() unexpected error: %v", err)
				}
				if scanner == nil {
					t.Error("NewScanner() returned nil scanner")
				}
			}
		})
	}
}

func TestScanner_ScanModule_MockOutput(t *testing.T) {
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "govulncheck")

	mockOutput := `{"osv":{"id":"GO-2025-0001","summary":"Test vulnerability","details":"This is a test vulnerability","database_specific":{"severity":"HIGH"},"affected":[{"package":{"name":"github.com/test/vulnerable","ecosystem":"Go"},"ranges":[{"type":"SEMVER","events":[{"introduced":"0"},{"fixed":"1.2.3"}]}]}]}}
`

	scriptContent := `#!/bin/sh
echo '` + strings.TrimSpace(mockOutput) + `'
exit 1
`

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", tmpDir+":"+originalPath)

	scanner := &Scanner{}
	ctx := context.Background()

	result, err := scanner.ScanModule(ctx, ".")
	if err != nil {
		t.Fatalf("ScanModule() error: %v", err)
	}

	if result == nil {
		t.Fatal("ScanModule() returned nil result")
	}

	if len(result.Vulnerabilities) != 1 {
		t.Errorf("Expected 1 vulnerability, got %d", len(result.Vulnerabilities))
	}

	if len(result.Vulnerabilities) > 0 {
		vuln := result.Vulnerabilities[0]
		if vuln.ID != "GO-2025-0001" {
			t.Errorf("Vulnerability ID = %q, want %q", vuln.ID, "GO-2025-0001")
		}
		if vuln.Package != "github.com/test/vulnerable" {
			t.Errorf("Package = %q, want %q", vuln.Package, "github.com/test/vulnerable")
		}
		if vuln.Severity != "HIGH" {
			t.Errorf("Severity = %q, want %q", vuln.Severity, "HIGH")
		}
		if vuln.Fixed != "1.2.3" {
			t.Errorf("Fixed = %q, want %q", vuln.Fixed, "1.2.3")
		}
		if vuln.Description != "Test vulnerability" {
			t.Errorf("Description = %q, want %q", vuln.Description, "Test vulnerability")
		}
	}

	if result.TotalVulns != 1 {
		t.Errorf("TotalVulns = %d, want 1", result.TotalVulns)
	}

	if result.TotalScanned != 1 {
		t.Errorf("TotalScanned = %d, want 1", result.TotalScanned)
	}
}

func TestScanner_ScanModule_NoVulnerabilities(t *testing.T) {
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "govulncheck")

	scriptContent := `#!/bin/sh
echo '{"config":{"protocol_version":"v1.0.0"}}'
exit 0
`

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", tmpDir+":"+originalPath)

	scanner := &Scanner{}
	ctx := context.Background()

	result, err := scanner.ScanModule(ctx, ".")
	if err != nil {
		t.Fatalf("ScanModule() error: %v", err)
	}

	if len(result.Vulnerabilities) != 0 {
		t.Errorf("Expected 0 vulnerabilities, got %d", len(result.Vulnerabilities))
	}

	if result.TotalVulns != 0 {
		t.Errorf("TotalVulns = %d, want 0", result.TotalVulns)
	}
}

func TestScanner_ScanModule_MultipleVulnerabilities(t *testing.T) {
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "govulncheck")

	mockOutput := `{"osv":{"id":"GO-2025-0001","summary":"First vuln","database_specific":{"severity":"HIGH"},"affected":[{"package":{"name":"github.com/test/pkg1"},"ranges":[{"type":"SEMVER","events":[{"fixed":"1.0.0"}]}]}]}}
{"osv":{"id":"GO-2025-0002","summary":"Second vuln","database_specific":{"severity":"MODERATE"},"affected":[{"package":{"name":"github.com/test/pkg2"},"ranges":[{"type":"SEMVER","events":[{"fixed":"2.0.0"}]}]}]}}
{"osv":{"id":"GO-2025-0003","summary":"Third vuln","database_specific":{"severity":"LOW"},"affected":[{"package":{"name":"github.com/test/pkg3"},"ranges":[{"type":"SEMVER","events":[{"fixed":"3.0.0"}]}]}]}}
`

	scriptContent := `#!/bin/sh
cat << 'EOF'
` + mockOutput + `EOF
exit 1
`

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", tmpDir+":"+originalPath)

	scanner := &Scanner{}
	ctx := context.Background()

	result, err := scanner.ScanModule(ctx, ".")
	if err != nil {
		t.Fatalf("ScanModule() error: %v", err)
	}

	if len(result.Vulnerabilities) != 3 {
		t.Errorf("Expected 3 vulnerabilities, got %d", len(result.Vulnerabilities))
	}

	if result.TotalVulns != 3 {
		t.Errorf("TotalVulns = %d, want 3", result.TotalVulns)
	}

	severityMap := make(map[string]int)
	for _, v := range result.Vulnerabilities {
		severityMap[v.Severity]++
	}

	if severityMap["HIGH"] != 1 {
		t.Errorf("Expected 1 HIGH severity, got %d", severityMap["HIGH"])
	}
	if severityMap["MODERATE"] != 1 {
		t.Errorf("Expected 1 MODERATE severity, got %d", severityMap["MODERATE"])
	}
	if severityMap["LOW"] != 1 {
		t.Errorf("Expected 1 LOW severity, got %d", severityMap["LOW"])
	}
}

func TestScanner_ScanModule_UnknownSeverity(t *testing.T) {
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "govulncheck")

	mockOutput := `{"osv":{"id":"GO-2025-0001","summary":"Test","affected":[{"package":{"name":"github.com/test/pkg"},"ranges":[{"type":"SEMVER","events":[{"fixed":"1.0.0"}]}]}]}}
`

	scriptContent := `#!/bin/sh
echo '` + strings.TrimSpace(mockOutput) + `'
exit 1
`

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", tmpDir+":"+originalPath)

	scanner := &Scanner{}
	ctx := context.Background()

	result, err := scanner.ScanModule(ctx, ".")
	if err != nil {
		t.Fatalf("ScanModule() error: %v", err)
	}

	if len(result.Vulnerabilities) != 1 {
		t.Fatalf("Expected 1 vulnerability, got %d", len(result.Vulnerabilities))
	}

	vuln := result.Vulnerabilities[0]
	if vuln.Severity != "UNKNOWN" {
		t.Errorf("Severity = %q, want %q", vuln.Severity, "UNKNOWN")
	}
}

func TestScanner_ScanModule_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping context cancellation test in short mode")
	}

	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "govulncheck")

	scriptContent := `#!/bin/sh
sleep 10
`

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", tmpDir+":"+originalPath)

	scanner := &Scanner{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := scanner.ScanModule(ctx, ".")
	if err == nil {
		t.Error("ScanModule() should error when context is cancelled")
	}
}

func TestScanner_ScanModule_CommandFailure(t *testing.T) {
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "govulncheck")

	scriptContent := `#!/bin/sh
exit 2
`

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", tmpDir+":"+originalPath)

	scanner := &Scanner{}
	ctx := context.Background()

	_, err := scanner.ScanModule(ctx, ".")
	if err == nil {
		t.Error("ScanModule() should error when command fails with no output")
	}

	if !strings.Contains(err.Error(), "govulncheck failed") {
		t.Errorf("Error should mention 'govulncheck failed', got: %v", err)
	}
}

func TestScanner_ScanModule_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "govulncheck")

	scriptContent := `#!/bin/sh
echo 'invalid json line'
echo '{"osv":{"id":"GO-2025-0001","summary":"Valid","affected":[{"package":{"name":"test"},"ranges":[{"events":[{"fixed":"1.0.0"}]}]}]}}'
exit 1
`

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", tmpDir+":"+originalPath)

	scanner := &Scanner{}
	ctx := context.Background()

	result, err := scanner.ScanModule(ctx, ".")
	if err != nil {
		t.Fatalf("ScanModule() should skip invalid JSON lines: %v", err)
	}

	if len(result.Vulnerabilities) != 1 {
		t.Errorf("Expected 1 vulnerability (invalid JSON skipped), got %d", len(result.Vulnerabilities))
	}
}

func TestScanner_ScanModule_EmptyLines(t *testing.T) {
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "govulncheck")

	mockOutput := `
{"osv":{"id":"GO-2025-0001","summary":"Test","affected":[{"package":{"name":"test"},"ranges":[{"events":[{"fixed":"1.0.0"}]}]}]}}

{"osv":{"id":"GO-2025-0002","summary":"Test2","affected":[{"package":{"name":"test2"},"ranges":[{"events":[{"fixed":"2.0.0"}]}]}]}}

`

	scriptContent := `#!/bin/sh
cat << 'EOF'
` + mockOutput + `EOF
exit 1
`

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", tmpDir+":"+originalPath)

	scanner := &Scanner{}
	ctx := context.Background()

	result, err := scanner.ScanModule(ctx, ".")
	if err != nil {
		t.Fatalf("ScanModule() error: %v", err)
	}

	if len(result.Vulnerabilities) != 2 {
		t.Errorf("Expected 2 vulnerabilities, got %d", len(result.Vulnerabilities))
	}
}

func TestScanner_ScanModule_DuplicateVulnerabilities(t *testing.T) {
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "govulncheck")

	mockOutput := `{"osv":{"id":"GO-2025-0001","summary":"Test","affected":[{"package":{"name":"github.com/test/pkg"},"ranges":[{"events":[{"fixed":"1.0.0"}]}]}]}}
{"osv":{"id":"GO-2025-0001","summary":"Test","affected":[{"package":{"name":"github.com/test/pkg"},"ranges":[{"events":[{"fixed":"1.0.0"}]}]}]}}
`

	scriptContent := `#!/bin/sh
cat << 'EOF'
` + mockOutput + `EOF
exit 1
`

	if err := os.WriteFile(mockScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create mock script: %v", err)
	}

	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", tmpDir+":"+originalPath)

	scanner := &Scanner{}
	ctx := context.Background()

	result, err := scanner.ScanModule(ctx, ".")
	if err != nil {
		t.Fatalf("ScanModule() error: %v", err)
	}

	if len(result.Vulnerabilities) != 1 {
		t.Errorf("Expected 1 vulnerability (deduplicated), got %d", len(result.Vulnerabilities))
	}
}

func TestFilterBySeverity(t *testing.T) {
	vulns := []*Vulnerability{
		{ID: "V1", Severity: "CRITICAL"},
		{ID: "V2", Severity: "HIGH"},
		{ID: "V3", Severity: "MODERATE"},
		{ID: "V4", Severity: "LOW"},
		{ID: "V5", Severity: "HIGH"},
		{ID: "V6", Severity: "UNKNOWN"},
	}

	tests := []struct {
		name       string
		severities []string
		wantCount  int
		wantIDs    []string
	}{
		{
			name:       "filter CRITICAL",
			severities: []string{"CRITICAL"},
			wantCount:  1,
			wantIDs:    []string{"V1"},
		},
		{
			name:       "filter HIGH",
			severities: []string{"HIGH"},
			wantCount:  2,
			wantIDs:    []string{"V2", "V5"},
		},
		{
			name:       "filter multiple severities",
			severities: []string{"CRITICAL", "HIGH"},
			wantCount:  3,
			wantIDs:    []string{"V1", "V2", "V5"},
		},
		{
			name:       "filter LOW and MODERATE",
			severities: []string{"LOW", "MODERATE"},
			wantCount:  2,
			wantIDs:    []string{"V3", "V4"},
		},
		{
			name:       "empty filter returns all",
			severities: []string{},
			wantCount:  6,
			wantIDs:    []string{"V1", "V2", "V3", "V4", "V5", "V6"},
		},
		{
			name:       "nil filter returns all",
			severities: nil,
			wantCount:  6,
			wantIDs:    []string{"V1", "V2", "V3", "V4", "V5", "V6"},
		},
		{
			name:       "filter non-existent severity",
			severities: []string{"SUPER_CRITICAL"},
			wantCount:  0,
			wantIDs:    []string{},
		},
		{
			name:       "filter UNKNOWN",
			severities: []string{"UNKNOWN"},
			wantCount:  1,
			wantIDs:    []string{"V6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterBySeverity(vulns, tt.severities)

			if len(filtered) != tt.wantCount {
				t.Errorf("FilterBySeverity() returned %d vulnerabilities, want %d", len(filtered), tt.wantCount)
			}

			gotIDs := make(map[string]bool)
			for _, v := range filtered {
				gotIDs[v.ID] = true
			}

			for _, wantID := range tt.wantIDs {
				if !gotIDs[wantID] {
					t.Errorf("Expected vulnerability %q in filtered results", wantID)
				}
			}
		})
	}
}

func TestFilterBySeverity_EmptyInput(t *testing.T) {
	filtered := FilterBySeverity([]*Vulnerability{}, []string{"HIGH"})

	if len(filtered) != 0 {
		t.Errorf("FilterBySeverity() with empty input should return empty, got %d", len(filtered))
	}
}

func TestFilterBySeverity_PreservesOrder(t *testing.T) {
	vulns := []*Vulnerability{
		{ID: "V1", Severity: "HIGH"},
		{ID: "V2", Severity: "LOW"},
		{ID: "V3", Severity: "HIGH"},
		{ID: "V4", Severity: "LOW"},
	}

	filtered := FilterBySeverity(vulns, []string{"HIGH"})

	if len(filtered) != 2 {
		t.Fatalf("Expected 2 vulnerabilities, got %d", len(filtered))
	}

	if filtered[0].ID != "V1" {
		t.Errorf("filtered[0].ID = %q, want %q", filtered[0].ID, "V1")
	}
	if filtered[1].ID != "V3" {
		t.Errorf("filtered[1].ID = %q, want %q", filtered[1].ID, "V3")
	}
}

func TestVulnerability_Structure(t *testing.T) {
	vuln := &Vulnerability{
		ID:          "GO-2025-0001",
		Package:     "github.com/test/vulnerable",
		Severity:    "HIGH",
		Description: "Test vulnerability",
		Fixed:       "1.2.3",
		Installed:   "1.0.0",
		URL:         "https://pkg.go.dev/vuln/GO-2025-0001",
	}

	if vuln.ID != "GO-2025-0001" {
		t.Errorf("ID = %q, want %q", vuln.ID, "GO-2025-0001")
	}

	if vuln.Package != "github.com/test/vulnerable" {
		t.Errorf("Package = %q, want %q", vuln.Package, "github.com/test/vulnerable")
	}

	if vuln.Severity != "HIGH" {
		t.Errorf("Severity = %q, want %q", vuln.Severity, "HIGH")
	}

	if vuln.Description != "Test vulnerability" {
		t.Errorf("Description = %q, want %q", vuln.Description, "Test vulnerability")
	}

	if vuln.Fixed != "1.2.3" {
		t.Errorf("Fixed = %q, want %q", vuln.Fixed, "1.2.3")
	}

	if vuln.Installed != "1.0.0" {
		t.Errorf("Installed = %q, want %q", vuln.Installed, "1.0.0")
	}

	if vuln.URL != "https://pkg.go.dev/vuln/GO-2025-0001" {
		t.Errorf("URL = %q, want %q", vuln.URL, "https://pkg.go.dev/vuln/GO-2025-0001")
	}
}

func TestScanResult_Structure(t *testing.T) {
	result := &ScanResult{
		Vulnerabilities: []*Vulnerability{
			{ID: "V1"},
			{ID: "V2"},
		},
		TotalScanned: 10,
		TotalVulns:   2,
	}

	if len(result.Vulnerabilities) != 2 {
		t.Errorf("len(Vulnerabilities) = %d, want 2", len(result.Vulnerabilities))
	}

	if result.TotalScanned != 10 {
		t.Errorf("TotalScanned = %d, want 10", result.TotalScanned)
	}

	if result.TotalVulns != 2 {
		t.Errorf("TotalVulns = %d, want 2", result.TotalVulns)
	}
}

func TestGovulncheckMessage_JSONParsing(t *testing.T) {
	jsonData := `{
		"osv": {
			"id": "GO-2025-0001",
			"summary": "Test vulnerability",
			"details": "Detailed description",
			"database_specific": {
				"severity": "HIGH"
			},
			"affected": [
				{
					"package": {
						"name": "github.com/test/pkg",
						"ecosystem": "Go"
					},
					"ranges": [
						{
							"type": "SEMVER",
							"events": [
								{"introduced": "0"},
								{"fixed": "1.2.3"}
							]
						}
					]
				}
			]
		}
	}`

	var msg govulncheckMessage
	err := json.Unmarshal([]byte(jsonData), &msg)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if msg.OSV == nil {
		t.Fatal("OSV should not be nil")
	}

	if msg.OSV.ID != "GO-2025-0001" {
		t.Errorf("ID = %q, want %q", msg.OSV.ID, "GO-2025-0001")
	}

	if msg.OSV.Summary != "Test vulnerability" {
		t.Errorf("Summary = %q, want %q", msg.OSV.Summary, "Test vulnerability")
	}

	if msg.OSV.DatabaseSpecific == nil {
		t.Fatal("DatabaseSpecific should not be nil")
	}

	if msg.OSV.DatabaseSpecific.Severity != "HIGH" {
		t.Errorf("Severity = %q, want %q", msg.OSV.DatabaseSpecific.Severity, "HIGH")
	}

	if len(msg.OSV.Affected) != 1 {
		t.Fatalf("Expected 1 affected package, got %d", len(msg.OSV.Affected))
	}

	affected := msg.OSV.Affected[0]
	if affected.Package.Name != "github.com/test/pkg" {
		t.Errorf("Package name = %q, want %q", affected.Package.Name, "github.com/test/pkg")
	}
}

func BenchmarkFilterBySeverity(b *testing.B) {
	vulns := make([]*Vulnerability, 100)
	severities := []string{"CRITICAL", "HIGH", "MODERATE", "LOW", "UNKNOWN"}

	for i := 0; i < 100; i++ {
		vulns[i] = &Vulnerability{
			ID:       "V" + string(rune(i)),
			Severity: severities[i%len(severities)],
		}
	}

	b.ResetTimer()
	for b.Loop() {
		FilterBySeverity(vulns, []string{"HIGH", "CRITICAL"})
	}
}

func BenchmarkFilterBySeverity_EmptyFilter(b *testing.B) {
	vulns := make([]*Vulnerability, 100)
	for i := range 100 {
		vulns[i] = &Vulnerability{ID: "V" + string(rune(i))}
	}

	b.ResetTimer()
	for b.Loop() {
		FilterBySeverity(vulns, []string{})
	}
}
