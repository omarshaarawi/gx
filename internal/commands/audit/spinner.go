package audit

import (
	"context"

	"github.com/omarshaarawi/gx/internal/ui"
	"github.com/omarshaarawi/gx/internal/vulndb"
)

func scanModuleWithSpinner(ctx context.Context, scanner *vulndb.Scanner, modPath string) (*vulndb.ScanResult, error) {
	return ui.RunSimpleSpinner("Scanning for vulnerabilities...", func() (*vulndb.ScanResult, error) {
		return scanner.ScanModule(ctx, modPath)
	})
}
