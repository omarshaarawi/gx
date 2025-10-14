package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	HeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	CellStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	BorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	PatchStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))  // Green
	MinorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))  // Yellow
	MajorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))   // Red
	UpToDateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray
)

// Table represents a simple text table
type Table struct {
	Headers []string
	Rows    [][]string
	Widths  []int
}

// NewTable creates a new table with the given headers
func NewTable(headers ...string) *Table {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &Table{
		Headers: headers,
		Widths:  widths,
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) {
	if len(cells) != len(t.Headers) {
		return
	}

	for i, cell := range cells {
		if len(cell) > t.Widths[i] {
			t.Widths[i] = len(cell)
		}
	}

	t.Rows = append(t.Rows, cells)
}

// Render renders the table as a string
func (t *Table) Render() string {
	var b strings.Builder

	for i, header := range t.Headers {
		b.WriteString(HeaderStyle.Render(padRight(header, t.Widths[i])))
		if i < len(t.Headers)-1 {
			b.WriteString("  ")
		}
	}
	b.WriteString("\n")

	for i := range t.Headers {
		b.WriteString(strings.Repeat("─", t.Widths[i]))
		if i < len(t.Headers)-1 {
			b.WriteString("  ")
		}
	}
	b.WriteString("\n")

	for _, row := range t.Rows {
		for i, cell := range row {
			b.WriteString(CellStyle.Render(padRight(cell, t.Widths[i])))
			if i < len(row)-1 {
				b.WriteString("  ")
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// RenderStyled renders the table with custom cell styling
func (t *Table) RenderStyled(styleFunc func(rowIdx, colIdx int, cell string) lipgloss.Style) string {
	var b strings.Builder

	b.WriteString(BorderStyle.Render("┌"))
	for i := range t.Headers {
		b.WriteString(BorderStyle.Render(strings.Repeat("─", t.Widths[i]+2)))
		if i < len(t.Headers)-1 {
			b.WriteString(BorderStyle.Render("┬"))
		}
	}
	b.WriteString(BorderStyle.Render("┐"))
	b.WriteString("\n")

	b.WriteString(BorderStyle.Render("│ "))
	for i, header := range t.Headers {
		b.WriteString(HeaderStyle.Render(padRight(header, t.Widths[i])))
		b.WriteString(BorderStyle.Render(" │"))
		if i < len(t.Headers)-1 {
			b.WriteString(BorderStyle.Render(" "))
		}
	}
	b.WriteString("\n")

	b.WriteString(BorderStyle.Render("├"))
	for i := range t.Headers {
		b.WriteString(BorderStyle.Render(strings.Repeat("─", t.Widths[i]+2)))
		if i < len(t.Headers)-1 {
			b.WriteString(BorderStyle.Render("┼"))
		}
	}
	b.WriteString(BorderStyle.Render("┤"))
	b.WriteString("\n")

	for rowIdx, row := range t.Rows {
		b.WriteString(BorderStyle.Render("│ "))
		for colIdx, cell := range row {
			style := styleFunc(rowIdx, colIdx, cell)
			b.WriteString(style.Render(padRight(cell, t.Widths[colIdx])))
			b.WriteString(BorderStyle.Render(" │"))
			if colIdx < len(row)-1 {
				b.WriteString(BorderStyle.Render(" "))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(BorderStyle.Render("└"))
	for i := range t.Headers {
		b.WriteString(BorderStyle.Render(strings.Repeat("─", t.Widths[i]+2)))
		if i < len(t.Headers)-1 {
			b.WriteString(BorderStyle.Render("┴"))
		}
	}
	b.WriteString(BorderStyle.Render("┘"))
	b.WriteString("\n")

	return b.String()
}

// padRight pads a string to the right with spaces
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// TruncateString truncates a string to a maximum width with ellipsis
func TruncateString(s string, maxWidth int) string {
	if len(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return s[:maxWidth]
	}
	return s[:maxWidth-3] + "..."
}

// FormatVersionUpdate returns a styled version update string
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

// SimpleTable creates and renders a simple table in one call
func SimpleTable(headers []string, rows [][]string) string {
	t := NewTable(headers...)
	for _, row := range rows {
		t.AddRow(row...)
	}
	return t.Render()
}

// PrintTable prints a table to stdout
func PrintTable(t *Table) {
	fmt.Println(t.Render())
}
