package modfile

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Writer handles safe writing of go.mod files
type Writer struct {
	parser     *Parser
	backupMade bool
	backupPath string
}

// NewWriter creates a new modfile writer
func NewWriter(parser *Parser) *Writer {
	return &Writer{
		parser: parser,
	}
}

// Backup creates a timestamped backup of the go.mod file
func (w *Writer) Backup() error {
	if w.backupMade {
		return nil
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s.backup.%s", w.parser.path, timestamp)

	if err := os.WriteFile(backupPath, w.parser.data, 0o644); err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}

	w.backupMade = true
	w.backupPath = backupPath
	return nil
}

// RestoreBackup restores the backup file
func (w *Writer) RestoreBackup() error {
	if !w.backupMade {
		return fmt.Errorf("no backup to restore")
	}

	data, err := os.ReadFile(w.backupPath)
	if err != nil {
		return fmt.Errorf("reading backup: %w", err)
	}

	if err := os.WriteFile(w.parser.path, data, 0o644); err != nil {
		return fmt.Errorf("restoring backup: %w", err)
	}

	return nil
}

// CleanupBackup removes the backup file
func (w *Writer) CleanupBackup() error {
	if !w.backupMade {
		return nil
	}

	if err := os.Remove(w.backupPath); err != nil {
		return fmt.Errorf("removing backup: %w", err)
	}

	w.backupMade = false
	w.backupPath = ""
	return nil
}

// BackupPath returns the path to the backup file (if created)
func (w *Writer) BackupPath() string {
	return w.backupPath
}

// UpdateRequire updates or adds a requirement
func (w *Writer) UpdateRequire(modulePath, version string) error {
	if err := w.parser.file.AddRequire(modulePath, version); err != nil {
		return fmt.Errorf("updating require: %w", err)
	}
	return nil
}

// DropRequire removes a requirement
func (w *Writer) DropRequire(modulePath string) error {
	if err := w.parser.file.DropRequire(modulePath); err != nil {
		return fmt.Errorf("dropping require: %w", err)
	}
	return nil
}

// Format returns the formatted go.mod content
func (w *Writer) Format() ([]byte, error) {
	data, err := w.parser.file.Format()
	if err != nil {
		return nil, fmt.Errorf("formatting go.mod: %w", err)
	}
	return data, nil
}

// Write writes the formatted content to the go.mod file
func (w *Writer) Write() error {
	data, err := w.Format()
	if err != nil {
		return err
	}

	dir := filepath.Dir(w.parser.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	if err := os.WriteFile(w.parser.path, data, 0o644); err != nil {
		return fmt.Errorf("writing go.mod: %w", err)
	}

	return nil
}

// SafeWrite creates a backup, writes the file, and validates it
func (w *Writer) SafeWrite() error {
	if err := w.Backup(); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	if err := w.Write(); err != nil {
		if restoreErr := w.RestoreBackup(); restoreErr != nil {
			return fmt.Errorf("write failed and restore failed: %w (original error: %v)", restoreErr, err)
		}
		return fmt.Errorf("write failed (backup restored): %w", err)
	}

	if _, err := NewParser(w.parser.path); err != nil {
		if restoreErr := w.RestoreBackup(); restoreErr != nil {
			return fmt.Errorf("validation failed and restore failed: %w (original error: %v)", restoreErr, err)
		}
		return fmt.Errorf("validation failed (backup restored): %w", err)
	}

	return nil
}

// Cleanup calls modfile.Cleanup to remove empty sections
func (w *Writer) Cleanup() {
	w.parser.file.Cleanup()
}
