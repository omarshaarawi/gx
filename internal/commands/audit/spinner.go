package audit

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/omarshaarawi/gx/internal/vulndb"
)

type scanResult struct {
	result *vulndb.ScanResult
	err    error
}

type scanSpinnerModel struct {
	spinner spinner.Model
	message string
	done    bool
	err     error
	result  *vulndb.ScanResult
}

func newScanSpinnerModel(message string, _ chan scanResult) scanSpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return scanSpinnerModel{
		spinner: s,
		message: message,
	}
}

func (m scanSpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m scanSpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil

	case scanResult:
		m.done = true
		m.err = msg.err
		m.result = msg.result
		return m, tea.Quit

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m scanSpinnerModel) View() string {
	if m.done {
		return ""
	}

	return fmt.Sprintf("\n %s %s\n", m.spinner.View(), m.message)
}

func scanModuleWithSpinner(ctx context.Context, scanner *vulndb.Scanner, modPath string) (*vulndb.ScanResult, error) {
	m := newScanSpinnerModel("Scanning for vulnerabilities...", nil)
	p := tea.NewProgram(m)

	go func() {
		result, err := scanner.ScanModule(ctx, modPath)
		p.Send(scanResult{result: result, err: err})
	}()

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final := finalModel.(scanSpinnerModel)

	if final.err != nil {
		return nil, final.err
	}

	return final.result, nil
}

