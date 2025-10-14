package outdated

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarshaarawi/gx/internal/modfile"
	"github.com/omarshaarawi/gx/internal/proxy"
	"github.com/omarshaarawi/gx/internal/ui"
	xmodfile "golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

var (
	directHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	indirectHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("240"))
	summaryStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	ctaStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)

// Options configures the outdated command
type Options struct {
	DirectOnly bool
	MajorOnly  bool
	ModPath    string
}

// Package represents a package with version information
type Package struct {
	Name       string
	Current    string
	Latest     string
	UpdateType string // major, minor, patch, none
	Direct     bool
}

// Run executes the outdated command
func Run(opts Options) error {
	ctx := context.Background()

	parser, err := modfile.NewParser(opts.ModPath)
	if err != nil {
		return fmt.Errorf("parsing go.mod: %w", err)
	}

	proxyClient := proxy.NewClient("")

	var requires []*xmodfile.Require
	if opts.DirectOnly {
		requires = parser.DirectRequires()
	} else {
		requires = parser.AllRequires()
	}

	if len(requires) == 0 {
		fmt.Println("No dependencies found")
		return nil
	}

	packages, err := fetchPackagesWithSpinner(ctx, proxyClient, requires, opts)
	if err != nil {
		return fmt.Errorf("fetching packages: %w", err)
	}

	if len(packages) == 0 {
		fmt.Println("✨ All packages are up to date!")
		return nil
	}

	var directPkgs, indirectPkgs []Package
	for _, pkg := range packages {
		if pkg.Direct {
			directPkgs = append(directPkgs, pkg)
		} else {
			indirectPkgs = append(indirectPkgs, pkg)
		}
	}

	renderGroupedTables(directPkgs, indirectPkgs)

	return nil
}

// classifyUpdate determines the type of update (major, minor, patch, none)
func classifyUpdate(current, latest string) string {
	if semver.Compare(current, latest) >= 0 {
		return "none"
	}

	currentMajor := semver.Major(current)
	latestMajor := semver.Major(latest)

	if currentMajor != latestMajor {
		return "major"
	}

	currentParts := strings.Split(strings.TrimPrefix(current, currentMajor+"."), ".")
	latestParts := strings.Split(strings.TrimPrefix(latest, latestMajor+"."), ".")

	if len(currentParts) > 0 && len(latestParts) > 0 && currentParts[0] != latestParts[0] {
		return "minor"
	}

	return "patch"
}

// renderGroupedTables renders packages grouped by direct/indirect
func renderGroupedTables(directPkgs, indirectPkgs []Package) {
	maxNameWidth := 45

	if len(directPkgs) > 0 {
		fmt.Println(directHeaderStyle.Render("\n📦 Direct Dependencies"))
		fmt.Println()
		renderPackageTable(directPkgs, maxNameWidth)
	}

	if len(indirectPkgs) > 0 {
		fmt.Println(indirectHeaderStyle.Render("\n🔗 Indirect Dependencies"))
		fmt.Println()
		renderPackageTable(indirectPkgs, maxNameWidth)
	}

	totalPkgs := len(directPkgs) + len(indirectPkgs)
	major, minor, patch := 0, 0, 0
	for _, pkg := range append(directPkgs, indirectPkgs...) {
		switch pkg.UpdateType {
		case "major":
			major++
		case "minor":
			minor++
		case "patch":
			patch++
		}
	}

	fmt.Printf("\n%s ", summaryStyle.Render("📊 Summary:"))
	fmt.Printf("%d package(s) can be updated", totalPkgs)

	var parts []string
	if major > 0 {
		parts = append(parts, fmt.Sprintf("%s %d major", ui.MajorStyle.Render("●"), major))
	}
	if minor > 0 {
		parts = append(parts, fmt.Sprintf("%s %d minor", ui.MinorStyle.Render("●"), minor))
	}
	if patch > 0 {
		parts = append(parts, fmt.Sprintf("%s %d patch", ui.PatchStyle.Render("●"), patch))
	}

	if len(parts) > 0 {
		fmt.Printf(" (%s)", strings.Join(parts, ", "))
	}
	fmt.Println()

	fmt.Printf("\n💡 %s\n", ctaStyle.Render("Run `gx update -i` to choose which packages to update"))
}

// renderPackageTable renders a table of packages
func renderPackageTable(packages []Package, maxNameWidth int) {
	if len(packages) == 0 {
		return
	}

	table := ui.NewTable("Package", "Current", "Latest", "Update")

	for _, pkg := range packages {
		pkgName := ui.TruncateString(pkg.Name, maxNameWidth)

		symbol := ""
		switch pkg.UpdateType {
		case "major":
			symbol = "▲ "
		case "minor":
			symbol = "● "
		case "patch":
			symbol = "· "
		}

		table.AddRow(
			pkgName,
			pkg.Current,
			pkg.Latest,
			symbol+pkg.UpdateType,
		)
	}

	output := table.RenderStyled(func(rowIdx, colIdx int, cell string) lipgloss.Style {
		pkg := packages[rowIdx]

		switch colIdx {
		case 0:
			return ui.CellStyle

		case 1:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

		case 2:
			return ui.FormatVersionUpdate(pkg.UpdateType)

		case 3:
			return ui.FormatVersionUpdate(pkg.UpdateType)

		default:
			return ui.CellStyle
		}
	})

	fmt.Println(output)
}

