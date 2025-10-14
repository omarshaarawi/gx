package update

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/omarshaarawi/gx/internal/modfile"
	"github.com/omarshaarawi/gx/internal/proxy"
)

// Dependency represents a Go module dependency with version information
type Dependency struct {
	Name      string
	Current   string
	Target    string
	Latest    string
	LatestRaw string
	Direct    bool
	UpToDate  bool
}

// Options configures the update command
type Options struct {
	Interactive bool
	DryRun      bool
	All         bool
	Major       bool
	Vendor      bool
	ModPath     string
}

// Run executes the update command
func Run(ctx context.Context, opts Options) error {

	parser, err := modfile.NewParser(opts.ModPath)
	if err != nil {
		return fmt.Errorf("parsing go.mod: %w", err)
	}

	proxyClient := proxy.NewClient("")

	deps, err := loadDependenciesWithSpinner(ctx, parser, proxyClient)
	if err != nil {
		return fmt.Errorf("loading dependencies: %w", err)
	}

	if len(deps) == 0 {
		fmt.Println("No dependencies found in go.mod")
		return nil
	}

	allUpToDate := true
	for _, dep := range deps {
		if !dep.UpToDate {
			allUpToDate = false
			break
		}
	}

	if allUpToDate {
		fmt.Println("✨ All dependencies are up to date!")
		return nil
	}

	var toUpdate []*Dependency
	if opts.Interactive {
		selected, err := RunInteractive(deps)
		if err != nil {
			return fmt.Errorf("interactive selection: %w", err)
		}
		if selected == nil {
			fmt.Println("Update cancelled")
			return nil
		}
		toUpdate = selected
	} else if opts.All {
		for _, dep := range deps {
			if !dep.UpToDate {
				toUpdate = append(toUpdate, dep)
			}
		}
	} else {
		return fmt.Errorf("please specify -i (interactive) or --all")
	}

	if len(toUpdate) == 0 {
		fmt.Println("No packages selected for update")
		return nil
	}

	if opts.DryRun {
		fmt.Println("\n📋 Would update:")
		for _, dep := range toUpdate {
			fmt.Printf("  • %s: %s → %s\n", dep.Name, dep.Current, dep.Latest)
		}
		return nil
	}

	if err := updateDependenciesWithProgress(parser, toUpdate); err != nil {
		return fmt.Errorf("updating dependencies: %w", err)
	}

	fmt.Printf("\n✓ Successfully updated %d package(s)\n", len(toUpdate))

	fmt.Println("\n🔧 Running go mod tidy...")
	if err := runGoModTidy(); err != nil {
		fmt.Printf("⚠️  Warning: go mod tidy failed: %v\n", err)
		fmt.Println("   You may need to run 'go mod tidy' manually")
		return nil
	}
	fmt.Println("✓ go.mod and go.sum updated")

	if opts.Vendor {
		fmt.Println("\n📦 Running go mod vendor...")
		if err := runGoModVendor(); err != nil {
			fmt.Printf("⚠️  Warning: go mod vendor failed: %v\n", err)
			fmt.Println("   You may need to run 'go mod vendor' manually")
		} else {
			fmt.Println("✓ vendor directory updated")
		}
	}

	return nil
}

// runGoModTidy runs 'go mod tidy' to clean up dependencies
func runGoModTidy() error {
	cmd := exec.Command("go", "mod", "tidy")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// runGoModVendor runs 'go mod vendor' to update vendor directory
func runGoModVendor() error {
	cmd := exec.Command("go", "mod", "vendor")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
