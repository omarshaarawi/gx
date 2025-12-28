package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Table styles
	HeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	CellStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	BorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Version update styles
	PatchStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	MinorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	MajorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	UpToDateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Section header styles
	DirectHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	IndirectHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("240"))
	SummaryStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	CTAStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	// Severity styles
	CriticalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	HighStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	MediumStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	LowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
)

func SeverityStyle(severity string) lipgloss.Style {
	switch severity {
	case "CRITICAL":
		return CriticalStyle
	case "HIGH":
		return HighStyle
	case "MEDIUM":
		return MediumStyle
	case "LOW":
		return LowStyle
	default:
		return CellStyle
	}
}

func FormatVersionUpdate(updateType string) lipgloss.Style {
	switch updateType {
	case "major":
		return MajorStyle
	case "minor":
		return MinorStyle
	case "patch":
		return PatchStyle
	case "none":
		return UpToDateStyle
	default:
		return CellStyle
	}
}
