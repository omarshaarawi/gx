package outdated

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/omarshaarawi/gx/internal/proxy"
	xmodfile "golang.org/x/mod/modfile"
)

type fetchResult struct {
	packages []Package
	err      error
}

type spinnerModel struct {
	spinner  spinner.Model
	message  string
	total    int
	checked  int
	quitting bool
	err      error
	done     bool
	result   []Package
}

func newSpinnerModel(message string, total int, _ chan fetchResult) spinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return spinnerModel{
		spinner: s,
		message: message,
		total:   total,
	}
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

type progressMsg int

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case progressMsg:
		m.checked = int(msg)
		return m, nil

	case fetchResult:
		m.done = true
		m.err = msg.err
		m.result = msg.packages
		return m, tea.Quit

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m spinnerModel) View() string {
	if m.quitting {
		return ""
	}

	if m.done {
		if m.err != nil {
			return ""
		}
		return ""
	}

	return fmt.Sprintf("\n %s %s (%d/%d packages checked)\n",
		m.spinner.View(),
		m.message,
		m.checked,
		m.total,
	)
}

func fetchPackagesWithSpinner(ctx context.Context, proxyClient *proxy.Client, requires []*xmodfile.Require, opts Options) ([]Package, error) {
	progressCh := make(chan int, len(requires))
	m := newSpinnerModel("Checking for updates...", len(requires), nil)
	p := tea.NewProgram(m)

	go func() {
		for checked := range progressCh {
			p.Send(progressMsg(checked))
		}
	}()

	go func() {
		packages, err := fetchPackages(ctx, proxyClient, requires, opts, progressCh)
		close(progressCh)
		p.Send(fetchResult{packages: packages, err: err})
	}()

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final := finalModel.(spinnerModel)

	if final.quitting {
		return nil, fmt.Errorf("cancelled by user")
	}

	if final.err != nil {
		return nil, final.err
	}

	return final.result, nil
}

func fetchPackages(ctx context.Context, proxyClient *proxy.Client, requires []*xmodfile.Require, opts Options, progressCh chan<- int) ([]Package, error) {
	packages := []Package{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	checked := 0

	for _, req := range requires {
		wg.Add(1)
		go func(r *xmodfile.Require) {
			defer wg.Done()

			latest, err := proxyClient.Latest(ctx, r.Mod.Path)
			if err != nil {
				mu.Lock()
				checked++
				progressCh <- checked
				mu.Unlock()
				return
			}

			updateType := classifyUpdate(r.Mod.Version, latest.Version)

			if opts.MajorOnly && updateType != "major" {
				mu.Lock()
				checked++
				progressCh <- checked
				mu.Unlock()
				return
			}

			pkg := Package{
				Name:       r.Mod.Path,
				Current:    strings.TrimPrefix(r.Mod.Version, "v"),
				Latest:     strings.TrimPrefix(latest.Version, "v"),
				UpdateType: updateType,
				Direct:     !r.Indirect,
			}

			if updateType != "none" {
				mu.Lock()
				packages = append(packages, pkg)
				mu.Unlock()
			}

			mu.Lock()
			checked++
			progressCh <- checked
			mu.Unlock()
		}(req)
	}

	wg.Wait()
	return packages, nil
}

