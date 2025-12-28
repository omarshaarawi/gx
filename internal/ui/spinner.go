package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type spinnerResult[T any] struct {
	value T
	err   error
}

type spinnerModel[T any] struct {
	spinner  spinner.Model
	message  string
	total    int
	progress int
	done     bool
	result   spinnerResult[T]
}

func newSpinnerModel[T any](message string, total int) spinnerModel[T] {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return spinnerModel[T]{
		spinner: s,
		message: message,
		total:   total,
	}
}

func (m spinnerModel[T]) Init() tea.Cmd {
	return m.spinner.Tick
}

type progressMsg int

func (m spinnerModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.done = true
			m.result.err = fmt.Errorf("cancelled")
			return m, tea.Quit
		}
		return m, nil

	case progressMsg:
		m.progress = int(msg)
		return m, nil

	case spinnerResult[T]:
		m.done = true
		m.result = msg
		return m, tea.Quit

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m spinnerModel[T]) View() string {
	if m.done {
		return ""
	}

	if m.total > 0 {
		return fmt.Sprintf("\n %s %s (%d/%d)\n",
			m.spinner.View(),
			m.message,
			m.progress,
			m.total,
		)
	}

	return fmt.Sprintf("\n %s %s\n", m.spinner.View(), m.message)
}

type SpinnerTask[T any] struct {
	Message string
	Total   int
	Run     func(progress chan<- int) (T, error)
}

func RunWithSpinner[T any](task SpinnerTask[T]) (T, error) {
	m := newSpinnerModel[T](task.Message, task.Total)
	p := tea.NewProgram(m)

	progressCh := make(chan int, task.Total+1)

	go func() {
		for progress := range progressCh {
			p.Send(progressMsg(progress))
		}
	}()

	go func() {
		result, err := task.Run(progressCh)
		close(progressCh)
		p.Send(spinnerResult[T]{value: result, err: err})
	}()

	finalModel, err := p.Run()
	if err != nil {
		var zero T
		return zero, err
	}

	final := finalModel.(spinnerModel[T])
	return final.result.value, final.result.err
}

func RunSimpleSpinner[T any](message string, fn func() (T, error)) (T, error) {
	return RunWithSpinner(SpinnerTask[T]{
		Message: message,
		Run: func(_ chan<- int) (T, error) {
			return fn()
		},
	})
}
