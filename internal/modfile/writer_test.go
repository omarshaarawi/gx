package modfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	writerTestGoMod = `module omarshaarawi/testproject

go 1.24.2

require (
	github.com/stretchr/testify v1.8.4
	golang.org/x/mod v0.14.0
)
`

	writerMinimalGoMod = `module omarshaarawi/minimal

go 1.24.2
`
)

func TestNewWriter(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	if writer == nil {
		t.Fatal("NewWriter() returned nil")
	}

	if writer.parser != parser {
		t.Error("Writer.parser does not match input parser")
	}

	if writer.backupMade {
		t.Error("Writer.backupMade should be false initially")
	}

	if writer.backupPath != "" {
		t.Error("Writer.backupPath should be empty initially")
	}
}

func TestWriter_Backup(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	err = writer.Backup()
	if err != nil {
		t.Fatalf("Backup() unexpected error: %v", err)
	}

	if !writer.backupMade {
		t.Error("Writer.backupMade should be true after Backup()")
	}

	if writer.backupPath == "" {
		t.Fatal("Writer.backupPath should not be empty after Backup()")
	}

	if _, statErr := os.Stat(writer.backupPath); os.IsNotExist(statErr) {
		t.Errorf("Backup file does not exist at %s", writer.backupPath)
	}

	if !strings.Contains(writer.backupPath, ".backup.") {
		t.Errorf("Backup path %q should contain '.backup.'", writer.backupPath)
	}

	backupData, err := os.ReadFile(writer.backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup: %v", err)
	}

	if string(backupData) != string(parser.data) {
		t.Error("Backup content does not match original")
	}
}

func TestWriter_Backup_Idempotent(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	err = writer.Backup()
	if err != nil {
		t.Fatalf("First Backup() error: %v", err)
	}
	firstBackupPath := writer.backupPath

	time.Sleep(10 * time.Millisecond)

	err = writer.Backup()
	if err != nil {
		t.Fatalf("Second Backup() error: %v", err)
	}

	if writer.backupPath != firstBackupPath {
		t.Errorf("Second Backup() changed backup path from %q to %q", firstBackupPath, writer.backupPath)
	}
}

func TestWriter_BackupPath(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	if writer.BackupPath() != "" {
		t.Error("BackupPath() should return empty string before backup")
	}

	err = writer.Backup()
	if err != nil {
		t.Fatalf("Backup() error: %v", err)
	}

	backupPath := writer.BackupPath()
	if backupPath == "" {
		t.Error("BackupPath() should return non-empty string after backup")
	}

	if backupPath != writer.backupPath {
		t.Error("BackupPath() should return internal backupPath field")
	}
}

func TestWriter_RestoreBackup(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	err = writer.Backup()
	if err != nil {
		t.Fatalf("Backup() error: %v", err)
	}

	originalData := parser.data

	modifiedContent := "module modified\n\ngo 1.21\n"
	err = os.WriteFile(tmpFile, []byte(modifiedContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	err = writer.RestoreBackup()
	if err != nil {
		t.Fatalf("RestoreBackup() error: %v", err)
	}

	restoredData, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read restored file: %v", err)
	}

	if string(restoredData) != string(originalData) {
		t.Errorf("Restored content does not match original.\nGot:\n%s\nWant:\n%s", restoredData, originalData)
	}
}

func TestWriter_RestoreBackup_NoBackup(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	err = writer.RestoreBackup()
	if err == nil {
		t.Fatal("RestoreBackup() should return error when no backup exists")
	}

	if !strings.Contains(err.Error(), "no backup") {
		t.Errorf("RestoreBackup() error should mention 'no backup', got: %v", err)
	}
}

func TestWriter_CleanupBackup(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	err = writer.Backup()
	if err != nil {
		t.Fatalf("Backup() error: %v", err)
	}

	backupPath := writer.backupPath

	if _, statErr := os.Stat(backupPath); os.IsNotExist(statErr) {
		t.Fatal("Backup file should exist before cleanup")
	}

	err = writer.CleanupBackup()
	if err != nil {
		t.Fatalf("CleanupBackup() error: %v", err)
	}

	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("Backup file should not exist after cleanup")
	}

	if writer.backupMade {
		t.Error("Writer.backupMade should be false after cleanup")
	}

	if writer.backupPath != "" {
		t.Error("Writer.backupPath should be empty after cleanup")
	}
}

func TestWriter_CleanupBackup_NoBackup(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	err = writer.CleanupBackup()
	if err != nil {
		t.Errorf("CleanupBackup() should not error when no backup exists: %v", err)
	}
}

func TestWriter_UpdateRequire(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		version    string
		wantErr    bool
	}{
		{
			name:       "add new requirement",
			modulePath: "github.com/new/package",
			version:    "v1.0.0",
			wantErr:    false,
		},
		{
			name:       "update existing requirement",
			modulePath: "github.com/stretchr/testify",
			version:    "v1.9.0",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempGoMod(t, writerTestGoMod)
			parser, err := NewParser(tmpFile)
			if err != nil {
				t.Fatalf("NewParser() error: %v", err)
			}

			writer := NewWriter(parser)

			err = writer.UpdateRequire(tt.modulePath, tt.version)

			if tt.wantErr {
				if err == nil {
					t.Error("UpdateRequire() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateRequire() unexpected error: %v", err)
			}

			req := parser.FindRequire(tt.modulePath)
			if req == nil {
				t.Fatalf("FindRequire(%q) returned nil after UpdateRequire", tt.modulePath)
			}

			if req.Mod.Version != tt.version {
				t.Errorf("Updated version = %q, want %q", req.Mod.Version, tt.version)
			}
		})
	}
}

func TestWriter_DropRequire(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	if !parser.HasRequire("golang.org/x/mod") {
		t.Fatal("Test requires golang.org/x/mod to exist initially")
	}

	err = writer.DropRequire("golang.org/x/mod")
	if err != nil {
		t.Fatalf("DropRequire() error: %v", err)
	}

	if parser.HasRequire("golang.org/x/mod") {
		t.Error("Requirement should be removed after DropRequire")
	}
}

func TestWriter_DropRequire_NonExistent(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	err = writer.DropRequire("github.com/nonexistent/package")
	if err != nil {
		t.Errorf("DropRequire() for non-existent package should not error: %v", err)
	}
}

func TestWriter_Format(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	data, err := writer.Format()
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	if len(data) == 0 {
		t.Error("Format() returned empty data")
	}

	content := string(data)
	if !strings.Contains(content, "module") {
		t.Error("Formatted content should contain 'module'")
	}

	if !strings.Contains(content, "go ") {
		t.Error("Formatted content should contain 'go' directive")
	}
}

func TestWriter_Write(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	err = writer.UpdateRequire("github.com/new/package", "v1.0.0")
	if err != nil {
		t.Fatalf("UpdateRequire() error: %v", err)
	}

	err = writer.Write()
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	writtenData, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	content := string(writtenData)
	if !strings.Contains(content, "github.com/new/package") {
		t.Error("Written file should contain new package")
	}

	newParser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("Written file should be parseable: %v", err)
	}

	if !newParser.HasRequire("github.com/new/package") {
		t.Error("New parser should find the added requirement")
	}
}

func TestWriter_Write_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "deep", "go.mod")

	parser := &Parser{
		path: nestedPath,
		file: nil,
		data: []byte(writerMinimalGoMod),
	}

	tmpFile := createTempGoMod(t, writerMinimalGoMod)
	tempParser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}
	parser.file = tempParser.file

	writer := NewWriter(parser)

	err = writer.Write()
	if err != nil {
		t.Fatalf("Write() should create directories: %v", err)
	}

	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("Write() should create nested directories and file")
	}
}

func TestWriter_SafeWrite(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	err = writer.UpdateRequire("github.com/safe/package", "v1.2.0")
	if err != nil {
		t.Fatalf("UpdateRequire() error: %v", err)
	}

	err = writer.SafeWrite()
	if err != nil {
		t.Fatalf("SafeWrite() error: %v", err)
	}

	if !writer.backupMade {
		t.Error("SafeWrite() should create backup")
	}

	newParser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("File should be parseable after SafeWrite: %v", err)
	}

	if !newParser.HasRequire("github.com/safe/package") {
		t.Error("SafeWrite() should persist changes")
	}

	if _, err := os.Stat(writer.backupPath); os.IsNotExist(err) {
		t.Error("Backup file should exist after SafeWrite")
	}
}

func TestWriter_SafeWrite_RestoresOnValidationFailure(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	originalContent := string(parser.data)

	writer := NewWriter(parser)

	originalPath := parser.path
	parser.path = "/invalid/\x00/path/go.mod"

	err = writer.SafeWrite()
	if err == nil {
		t.Fatal("SafeWrite() should error with invalid path")
	}

	parser.path = originalPath

	errMsg := err.Error()
	if !strings.Contains(errMsg, "write") && !strings.Contains(errMsg, "directory") && !strings.Contains(errMsg, "creating") {
		t.Errorf("SafeWrite() error should mention write or directory failure, got: %v", err)
	}

	currentData, _ := os.ReadFile(tmpFile)
	if string(currentData) != originalContent {
		t.Error("Original file should not be modified when SafeWrite fails")
	}
}

func TestWriter_Cleanup(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	err = writer.UpdateRequire("github.com/temp/package", "v1.0.0")
	if err != nil {
		t.Fatalf("UpdateRequire() error: %v", err)
	}

	err = writer.DropRequire("github.com/temp/package")
	if err != nil {
		t.Fatalf("DropRequire() error: %v", err)
	}

	writer.Cleanup()

	data, err := writer.Format()
	if err != nil {
		t.Fatalf("Format() error after Cleanup: %v", err)
	}

	if len(data) == 0 {
		t.Error("Format() should return data after Cleanup()")
	}
}

func TestWriter_IntegrationWorkflow(t *testing.T) {
	tmpFile := createTempGoMod(t, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)

	if backupErr := writer.Backup(); backupErr != nil {
		t.Fatalf("Backup() error: %v", backupErr)
	}

	if updateErr := writer.UpdateRequire("github.com/workflow/test", "v1.2.3"); updateErr != nil {
		t.Fatalf("UpdateRequire() error: %v", updateErr)
	}

	if dropErr := writer.DropRequire("github.com/stretchr/testify"); dropErr != nil {
		t.Fatalf("DropRequire() error: %v", dropErr)
	}

	if writeErr := writer.Write(); writeErr != nil {
		t.Fatalf("Write() error: %v", writeErr)
	}

	modifiedParser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() after modifications: %v", err)
	}

	if !modifiedParser.HasRequire("github.com/workflow/test") {
		t.Error("Modified file should have new requirement")
	}

	if modifiedParser.HasRequire("github.com/stretchr/testify") {
		t.Error("Modified file should not have dropped requirement")
	}

	if restoreErr := writer.RestoreBackup(); restoreErr != nil {
		t.Fatalf("RestoreBackup() error: %v", restoreErr)
	}

	restoredParser, err := NewParser(tmpFile)
	if err != nil {
		t.Fatalf("NewParser() after restoration: %v", err)
	}

	if restoredParser.HasRequire("github.com/workflow/test") {
		t.Error("Restored file should not have new requirement")
	}

	if !restoredParser.HasRequire("github.com/stretchr/testify") {
		t.Error("Restored file should have original requirement")
	}

	if err := writer.CleanupBackup(); err != nil {
		t.Fatalf("CleanupBackup() error: %v", err)
	}

	if _, err := os.Stat(writer.backupPath); !os.IsNotExist(err) {
		t.Error("Backup file should be removed after cleanup")
	}
}


func BenchmarkWriter_Backup(b *testing.B) {
	tmpFile := createTempGoMod(b, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		b.Fatalf("NewParser() error: %v", err)
	}

	b.ResetTimer()
	for b.Loop(){
		writer := NewWriter(parser)
		_ = writer.Backup()
	}
}

func BenchmarkWriter_Write(b *testing.B) {
	tmpFile := createTempGoMod(b, writerTestGoMod)
	parser, err := NewParser(tmpFile)
	if err != nil {
		b.Fatalf("NewParser() error: %v", err)
	}

	writer := NewWriter(parser)
	_ = writer.UpdateRequire("github.com/bench/test", "v1.0.0")

	b.ResetTimer()
	for b.Loop() {
		_ = writer.Write()
	}
}

func BenchmarkWriter_SafeWrite(b *testing.B) {
	tmpDir := b.TempDir()

	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		tmpFile := filepath.Join(tmpDir, "go.mod")
		_ = os.WriteFile(tmpFile, []byte(writerTestGoMod), 0o644)
		parser, _ := NewParser(tmpFile)
		writer := NewWriter(parser)
		_ = writer.UpdateRequire("github.com/bench/test", "v1.0.0")
		b.StartTimer()

		_ = writer.SafeWrite()
	}
}
