package update

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/omarshaarawi/gx/internal/modfile"
	"github.com/omarshaarawi/gx/internal/proxy"
	xmodfile "golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

type fetchDepsResult struct {
	deps []*Dependency
	err  error
}

type loadSpinnerModel struct {
	spinner  spinner.Model
	message  string
	total    int
	loaded   int
	quitting bool
	err      error
	done     bool
	result   []*Dependency
}

func newLoadSpinnerModel(message string, total int, _ chan fetchDepsResult) loadSpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return loadSpinnerModel{
		spinner: s,
		message: message,
		total:   total,
	}
}

func (m loadSpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

type loadProgressMsg int

func (m loadSpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case loadProgressMsg:
		m.loaded = int(msg)
		return m, nil

	case fetchDepsResult:
		m.done = true
		m.err = msg.err
		m.result = msg.deps
		return m, tea.Quit

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m loadSpinnerModel) View() string {
	if m.quitting {
		return ""
	}

	if m.done {
		return ""
	}

	return fmt.Sprintf("\n %s %s (%d/%d dependencies loaded)\n",
		m.spinner.View(),
		m.message,
		m.loaded,
		m.total,
	)
}

func loadDependenciesWithSpinner(ctx context.Context, parser *modfile.Parser, client *proxy.Client) ([]*Dependency, error) {
	allReqs := parser.AllRequires()
	if len(allReqs) == 0 {
		return nil, nil
	}

	progressCh := make(chan int, len(allReqs))
	m := newLoadSpinnerModel("Checking for updates...", len(allReqs), nil)
	p := tea.NewProgram(m)

	go func() {
		for loaded := range progressCh {
			p.Send(loadProgressMsg(loaded))
		}
	}()

	go func() {
		deps, err := fetchDependenciesParallel(ctx, allReqs, client, progressCh)
		close(progressCh)
		p.Send(fetchDepsResult{deps: deps, err: err})
	}()

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final := finalModel.(loadSpinnerModel)

	if final.quitting {
		return nil, fmt.Errorf("cancelled by user")
	}

	if final.err != nil {
		return nil, final.err
	}

	return final.result, nil
}

func fetchDependenciesParallel(ctx context.Context, allReqs []*xmodfile.Require, client *proxy.Client, progressCh chan<- int) ([]*Dependency, error) {
	deps := make([]*Dependency, len(allReqs))
	var wg sync.WaitGroup
	var mu sync.Mutex
	loaded := 0

	for i, req := range allReqs {
		wg.Add(1)
		go func(idx int, r *xmodfile.Require) {
			defer wg.Done()

			latest, err := client.Latest(ctx, r.Mod.Path)
			if err != nil {
				latest = &proxy.VersionInfo{Version: "unknown"}
			}

			target := latest.Version
			upToDate := false
			if semver.Compare(r.Mod.Version, latest.Version) >= 0 {
				target = r.Mod.Version
				upToDate = true
			}

			dep := &Dependency{
				Name:      r.Mod.Path,
				Current:   strings.TrimPrefix(r.Mod.Version, "v"),
				Target:    strings.TrimPrefix(target, "v"),
				Latest:    strings.TrimPrefix(latest.Version, "v"),
				LatestRaw: latest.Version,
				Direct:    !r.Indirect,
				UpToDate:  upToDate,
			}

			mu.Lock()
			deps[idx] = dep
			loaded++
			progressCh <- loaded
			mu.Unlock()
		}(i, req)
	}

	wg.Wait()
	return deps, nil
}

type updateProgress struct {
	current int
	total   int
	pkgName string
	status  string
}

type updateProgressModel struct {
	spinner  spinner.Model
	progress updateProgress
	done     bool
	resultCh chan error
}

func newUpdateProgressModel(total int, resultCh chan error) updateProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return updateProgressModel{
		spinner:  s,
		progress: updateProgress{total: total},
		resultCh: resultCh,
	}
}

func (m updateProgressModel) Init() tea.Cmd {
	return m.spinner.Tick
}

type updateProgressMsg updateProgress

func (m updateProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil

	case updateProgressMsg:
		m.progress = updateProgress(msg)
		return m, nil

	case error:
		m.done = true
		return m, tea.Quit

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m updateProgressModel) View() string {
	if m.done {
		return ""
	}

	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	return fmt.Sprintf("\n %s Updating go.mod... (%d/%d)\n   %s\n   %s\n",
		m.spinner.View(),
		m.progress.current,
		m.progress.total,
		m.progress.pkgName,
		statusStyle.Render(m.progress.status),
	)
}

func updateDependenciesWithProgress(parser *modfile.Parser, deps []*Dependency) error {
	resultCh := make(chan error, 1)
	progressCh := make(chan updateProgress, len(deps))

	go func() {
		err := performUpdates(parser, deps, progressCh)
		resultCh <- err
	}()

	m := newUpdateProgressModel(len(deps), resultCh)
	p := tea.NewProgram(m)

	go func() {
		for progress := range progressCh {
			p.Send(updateProgressMsg(progress))
		}
	}()

	_, err := p.Run()
	if err != nil {
		return err
	}

	result := <-resultCh
	close(progressCh)

	return result
}

func performUpdates(parser *modfile.Parser, deps []*Dependency, progressCh chan<- updateProgress) error {
	writer := modfile.NewWriter(parser)

	if err := writer.Backup(); err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}

	for i, dep := range deps {
		progressCh <- updateProgress{
			current: i + 1,
			total:   len(deps),
			pkgName: dep.Name,
			status:  fmt.Sprintf("%s → %s", dep.Current, dep.Latest),
		}

		if err := writer.UpdateRequire(dep.Name, dep.LatestRaw); err != nil {
			writer.RestoreBackup()
			return fmt.Errorf("updating %s: %w", dep.Name, err)
		}
	}

	writer.Cleanup()

	if err := writer.SafeWrite(); err != nil {
		return fmt.Errorf("writing go.mod: %w", err)
	}

	if err := writer.CleanupBackup(); err != nil {
		return fmt.Errorf("cleanup backup: %w", err)
	}

	return nil
}

