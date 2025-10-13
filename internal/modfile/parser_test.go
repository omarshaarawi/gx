package modfile

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	validGoMod = `module omarshaarawi/gx

go 1.24.2

require (
	github.com/stretchr/testify v1.8.4
	golang.org/x/mod v0.14.0
	omarshaarawi/direct v1.0.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
`

	minimalGoMod = `module omarshaarawi/minimal

go 1.24.2
`

	noModuleGoMod = `go 1.24.2

require (
	github.com/some/package v1.0.0
)
`

	invalidGoMod = `this is not a valid go.mod file
module broken
`
)

func TestNewParser(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid go.mod with dependencies",
			content: validGoMod,
			wantErr: false,
		},
		{
			name:    "minimal go.mod",
			content: minimalGoMod,
			wantErr: false,
		},
		{
			name:    "no module directive",
			content: noModuleGoMod,
			wantErr: false,
		},
		{
			name:        "invalid go.mod syntax",
			content:     invalidGoMod,
			wantErr:     true,
			errContains: "parsing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempGoMod(t, tt.content)

			parser, err := NewParser(tmpFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewParser() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewParser() error = %v, should contain %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewParser() unexpected error: %v", err)
			}

			if parser == nil {
				t.Fatal("NewParser() returned nil parser")
			}

			if parser.file == nil {
				t.Error("Parser.file is nil")
			}

			if parser.path != tmpFile {
				t.Errorf("Parser.path = %q, want %q", parser.path, tmpFile)
			}

			if len(parser.data) == 0 {
				t.Error("Parser.data is empty")
			}
		})
	}
}

func TestNewParser_FileNotFound(t *testing.T) {
	parser, err := NewParser("/nonexistent/path/go.mod")

	if err == nil {
		t.Fatal("NewParser() expected error for nonexistent file, got nil")
	}

	if parser != nil {
		t.Errorf("NewParser() expected nil parser, got %v", parser)
	}

	if !contains(err.Error(), "reading") {
		t.Errorf("NewParser() error should contain 'reading', got: %v", err)
	}
}

func TestParser_File(t *testing.T) {
	tmpFile := createTempGoMod(t, validGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	file := parser.File()

	if file == nil {
		t.Fatal("File() returned nil")
	}

	if file != parser.file {
		t.Error("File() did not return the underlying file")
	}
}

func TestParser_ModulePath(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "standard module path",
			content: validGoMod,
			want:    "omarshaarawi/gx",
		},
		{
			name:    "minimal module path",
			content: minimalGoMod,
			want:    "omarshaarawi/minimal",
		},
		{
			name:    "no module directive",
			content: noModuleGoMod,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempGoMod(t, tt.content)
			parser, err := NewParser(tmpFile)
			if err != nil {
				t.Fatalf("NewParser() error: %v", err)
			}

			got := parser.ModulePath()

			if got != tt.want {
				t.Errorf("ModulePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParser_DirectRequires(t *testing.T) {
	tmpFile := createTempGoMod(t, validGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	direct := parser.DirectRequires()

	expectedDirect := []string{
		"github.com/stretchr/testify",
		"golang.org/x/mod",
		"omarshaarawi/direct",
	}

	if len(direct) != len(expectedDirect) {
		t.Fatalf("DirectRequires() returned %d requires, want %d", len(direct), len(expectedDirect))
	}

	for i, req := range direct {
		if req.Indirect {
			t.Errorf("DirectRequires()[%d] has Indirect=true, want false", i)
		}
		if req.Mod.Path != expectedDirect[i] {
			t.Errorf("DirectRequires()[%d] path = %q, want %q", i, req.Mod.Path, expectedDirect[i])
		}
	}
}

func TestParser_IndirectRequires(t *testing.T) {
	tmpFile := createTempGoMod(t, validGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	indirect := parser.IndirectRequires()

	expectedIndirect := []string{
		"github.com/davecgh/go-spew",
		"github.com/pmezard/go-difflib",
		"gopkg.in/yaml.v3",
	}

	if len(indirect) != len(expectedIndirect) {
		t.Fatalf("IndirectRequires() returned %d requires, want %d", len(indirect), len(expectedIndirect))
	}

	for i, req := range indirect {
		if !req.Indirect {
			t.Errorf("IndirectRequires()[%d] has Indirect=false, want true", i)
		}
		if req.Mod.Path != expectedIndirect[i] {
			t.Errorf("IndirectRequires()[%d] path = %q, want %q", i, req.Mod.Path, expectedIndirect[i])
		}
	}
}

func TestParser_AllRequires(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantCount int
	}{
		{
			name:      "full go.mod",
			content:   validGoMod,
			wantCount: 6,
		},
		{
			name:      "minimal go.mod",
			content:   minimalGoMod,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempGoMod(t, tt.content)
			parser, err := NewParser(tmpFile)
			if err != nil {
				t.Fatalf("NewParser() error: %v", err)
			}

			all := parser.AllRequires()

			if len(all) != tt.wantCount {
				t.Errorf("AllRequires() returned %d requires, want %d", len(all), tt.wantCount)
			}
		})
	}
}

func TestParser_FindRequire(t *testing.T) {
	tmpFile := createTempGoMod(t, validGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	tests := []struct {
		name       string
		modulePath string
		wantFound  bool
		wantPath   string
		wantVer    string
	}{
		{
			name:       "find direct dependency",
			modulePath: "github.com/stretchr/testify",
			wantFound:  true,
			wantPath:   "github.com/stretchr/testify",
			wantVer:    "v1.8.4",
		},
		{
			name:       "find indirect dependency",
			modulePath: "gopkg.in/yaml.v3",
			wantFound:  true,
			wantPath:   "gopkg.in/yaml.v3",
			wantVer:    "v3.0.1",
		},
		{
			name:       "module not found",
			modulePath: "github.com/nonexistent/package",
			wantFound:  false,
		},
		{
			name:       "empty module path",
			modulePath: "",
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := parser.FindRequire(tt.modulePath)

			if tt.wantFound {
				if req == nil {
					t.Fatal("FindRequire() returned nil, want non-nil")
				}
				if req.Mod.Path != tt.wantPath {
					t.Errorf("FindRequire() path = %q, want %q", req.Mod.Path, tt.wantPath)
				}
				if req.Mod.Version != tt.wantVer {
					t.Errorf("FindRequire() version = %q, want %q", req.Mod.Version, tt.wantVer)
				}
			} else {
				if req != nil {
					t.Errorf("FindRequire() = %v, want nil", req)
				}
			}
		})
	}
}

func TestParser_HasRequire(t *testing.T) {
	tmpFile := createTempGoMod(t, validGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	tests := []struct {
		name       string
		modulePath string
		want       bool
	}{
		{
			name:       "has direct dependency",
			modulePath: "golang.org/x/mod",
			want:       true,
		},
		{
			name:       "has indirect dependency",
			modulePath: "github.com/davecgh/go-spew",
			want:       true,
		},
		{
			name:       "does not have dependency",
			modulePath: "github.com/missing/package",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.HasRequire(tt.modulePath)

			if got != tt.want {
				t.Errorf("HasRequire(%q) = %v, want %v", tt.modulePath, got, tt.want)
			}
		})
	}
}

func TestParser_ConsistencyBetweenMethods(t *testing.T) {
	tmpFile := createTempGoMod(t, validGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	t.Run("direct + indirect = all", func(t *testing.T) {
		direct := parser.DirectRequires()
		indirect := parser.IndirectRequires()
		all := parser.AllRequires()

		total := len(direct) + len(indirect)
		if total != len(all) {
			t.Errorf("len(DirectRequires) + len(IndirectRequires) = %d, want %d", total, len(all))
		}
	})

	t.Run("FindRequire consistent with HasRequire", func(t *testing.T) {
		testPath := "golang.org/x/mod"

		found := parser.FindRequire(testPath)
		has := parser.HasRequire(testPath)

		if (found != nil) != has {
			t.Errorf("FindRequire(%q) != nil is %v, but HasRequire() is %v", testPath, found != nil, has)
		}
	})

	t.Run("all requires findable", func(t *testing.T) {
		all := parser.AllRequires()

		for _, req := range all {
			found := parser.FindRequire(req.Mod.Path)
			if found == nil {
				t.Errorf("FindRequire(%q) returned nil, but module is in AllRequires()", req.Mod.Path)
			}
			if !parser.HasRequire(req.Mod.Path) {
				t.Errorf("HasRequire(%q) returned false, but module is in AllRequires()", req.Mod.Path)
			}
		}
	})
}

func createTempGoMod(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "go.mod")

	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create temp go.mod: %v", err)
	}

	return tmpFile
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

func BenchmarkNewParser(b *testing.B) {
	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(tmpFile, []byte(validGoMod), 0o644); err != nil {
		b.Fatalf("Failed to create temp go.mod: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		_, err := NewParser(tmpFile)
		if err != nil {
			b.Fatalf("NewParser() error: %v", err)
		}
	}
}

func BenchmarkParser_DirectRequires(b *testing.B) {
	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(tmpFile, []byte(validGoMod), 0o644); err != nil {
		b.Fatalf("Failed to create temp go.mod: %v", err)
	}

	parser, err := NewParser(tmpFile)
	if err != nil {
		b.Fatalf("NewParser() error: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		_ = parser.DirectRequires()
	}
}

func BenchmarkParser_FindRequire(b *testing.B) {
	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(tmpFile, []byte(validGoMod), 0o644); err != nil {
		b.Fatalf("Failed to create temp go.mod: %v", err)
	}

	parser, err := NewParser(tmpFile)
	if err != nil {
		b.Fatalf("NewParser() error: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		_ = parser.FindRequire("golang.org/x/mod")
	}
}
