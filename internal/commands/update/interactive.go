// This file was generated with assistance from Qwen3-Coder
// https://github.com/QwenLM/Qwen3-Coder

package update

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2).Bold(true)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("241"))
	currentStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("white"))
	targetStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("green"))
	latestStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("yellow"))
	directStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("green"))
	indirectStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("yellow"))
	dimmedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	pkgNameStyle    = lipgloss.NewStyle().Width(40).MaxWidth(40)
	versionStyle    = lipgloss.NewStyle().Width(15).MaxWidth(15)
	dimmedPkgStyle  = lipgloss.NewStyle().Width(40).MaxWidth(40).Foreground(lipgloss.Color("240"))
)

type item struct {
	dep      *Dependency
	selected bool
}

func (i item) FilterValue() string { return i.dep.Name }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	var checkbox string
	if i.dep.UpToDate {
		checkbox = "   "
	} else if i.selected {
		checkbox = "◉"
	} else {
		checkbox = "○"
	}

	var depType string
	if i.dep.Direct {
		depType = directStyle.Render("●")
	} else {
		depType = dimmedStyle.Render("○")
	}

	var pkgRendered string
	if i.dep.UpToDate {
		pkgRendered = dimmedPkgStyle.Render(i.dep.Name)
	} else {
		pkgRendered = pkgNameStyle.Render(i.dep.Name)
	}

	row := fmt.Sprintf("%s %s %s %s %s %s",
		checkbox,
		depType,
		pkgRendered,
		currentStyle.Render(versionStyle.Render(i.dep.Current)),
		targetStyle.Render(versionStyle.Render(i.dep.Target)),
		latestStyle.Render(versionStyle.Render(i.dep.Latest)),
	)

	if index == m.Index() {
		fmt.Fprint(w, selectedItemStyle.Render("> "+row))
	} else {
		fmt.Fprint(w, itemStyle.Render("  "+row))
	}
}

type model struct {
	list         list.Model
	dependencies []*Dependency
	quitting     bool
	confirmed    bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c", "q"))):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys(" "))):
			if i, ok := m.list.SelectedItem().(item); ok && !i.dep.UpToDate {
				i.selected = !i.selected
				m.list.SetItem(m.list.Index(), i)
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
			items := m.list.Items()
			for idx, listItem := range items {
				if i, ok := listItem.(item); ok && !i.dep.UpToDate {
					i.selected = true
					m.list.SetItem(idx, i)
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
			items := m.list.Items()
			for idx, listItem := range items {
				if i, ok := listItem.(item); ok {
					i.selected = false
					m.list.SetItem(idx, i)
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("i"))):
			items := m.list.Items()
			for idx, listItem := range items {
				if i, ok := listItem.(item); ok && !i.dep.UpToDate {
					i.selected = !i.selected
					m.list.SetItem(idx, i)
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			m.confirmed = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 4)
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	titleText := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Render("📦 Select packages to update")

	helpText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("Space to toggle • Enter to confirm • a select all • n select none • i invert • q quit")

	legend := fmt.Sprintf("  %s direct  %s indirect",
		directStyle.Render("●"),
		dimmedStyle.Render("○"),
	)

	columnHeader := fmt.Sprintf("      %s %s %s %s",
		headerStyle.Render(pkgNameStyle.Render("Package")),
		headerStyle.Render(versionStyle.Render("Current")),
		headerStyle.Render(versionStyle.Render("Target")),
		headerStyle.Render(versionStyle.Render("Latest")),
	)

	header := lipgloss.JoinVertical(lipgloss.Left,
		"",
		titleText,
		helpText,
		"",
		legend,
		"",
		columnHeader,
	)

	return header + "\n" + m.list.View()
}

func RunInteractive(deps []*Dependency) ([]*Dependency, error) {
	var directDeps, indirectDeps []*Dependency
	for _, dep := range deps {
		if dep.Direct {
			directDeps = append(directDeps, dep)
		} else {
			indirectDeps = append(indirectDeps, dep)
		}
	}

	sortedDeps := append(directDeps, indirectDeps...)

	items := make([]list.Item, len(sortedDeps))
	for i, dep := range sortedDeps {
		items[i] = item{dep: dep, selected: false}
	}

	const defaultWidth = 120
	const defaultHeight = 30

	l := list.New(items, itemDelegate{}, defaultWidth, defaultHeight)
	l.Title = ""
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = titleStyle

	m := model{
		list:         l,
		dependencies: deps,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("running interactive UI: %w", err)
	}

	result := finalModel.(model)
	if result.quitting && !result.confirmed {
		return nil, nil
	}

	var selected []*Dependency
	for _, listItem := range result.list.Items() {
		if i, ok := listItem.(item); ok && i.selected {
			selected = append(selected, i.dep)
		}
	}

	return selected, nil
}
