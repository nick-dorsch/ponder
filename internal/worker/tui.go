package worker

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nick-dorsch/ponder/internal/ui/components"
)

var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 1)

	promptStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, true, true, false).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1).
			Margin(1, 0)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
)

type TUIModel struct {
	ModelName     string
	Iterations    int
	MaxIterations int
	CurrentTask   string
	Prompt        string
	History       *components.CompletedTasks
	Output        *components.WorkerOutput
	ready         bool
	expanded      bool
	width         int
	height        int
	headerHeight  int
	promptHeight  int
	historyHeight int
	err           error
}

func NewTUIModel(modelName string, maxIterations int) *TUIModel {
	return &TUIModel{
		ModelName:     modelName,
		MaxIterations: maxIterations,
		History:       components.NewCompletedTasks(0),
		Output:        components.NewWorkerOutput(0, 0),
	}
}

func (m TUIModel) Init() tea.Cmd {
	return nil
}

type OutputMsg string
type StatusMsg string
type TaskMsg struct {
	Name   string
	Prompt string
}
type TaskResultMsg components.TaskResult
type IterationMsg int

func (m *TUIModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	if !m.ready {
		m.Output.SetSize(width, 0)
		m.ready = true
	}
	m.History.Width = width
	m.recalculateLayout()
}

func (m *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "e", tea.KeyEnter.String():
			m.expanded = !m.expanded
			m.recalculateLayout()
		}

	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft {
			// Check if the click is within the viewport Y bounds
			startY := m.headerHeight + m.promptHeight + 2
			if m.historyHeight > 0 {
				startY = m.headerHeight + m.historyHeight + m.promptHeight + 4
			}
			endY := startY + m.Output.Height()
			if msg.Y >= startY && msg.Y < endY {
				m.expanded = !m.expanded
				m.recalculateLayout()
			}
		} else if msg.Type == tea.MouseWheelUp || msg.Type == tea.MouseWheelDown {
		}

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case OutputMsg:
		m.Output.Append(string(msg))

	case StatusMsg:
		m.Output.AppendStatus(string(msg))

	case TaskMsg:
		m.CurrentTask = msg.Name
		m.Prompt = msg.Prompt
		m.Output.Reset()
		m.recalculateLayout()

	case TaskResultMsg:
		m.History.Add(components.TaskResult(msg), 5)
		m.recalculateLayout()

	case IterationMsg:
		m.Iterations = int(msg)
		m.recalculateLayout()

	case error:
		m.err = msg
		return m, tea.Quit
	}

	cmd = m.Output.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *TUIModel) recalculateLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	header := headerStyle.Render(fmt.Sprintf("Ponder Worker | Model: %s | Iteration: %d/%d", m.ModelName, m.Iterations, m.MaxIterations))
	m.headerHeight = lipgloss.Height(header)

	promptContent := fmt.Sprintf("Task: %s\n\n%s", m.CurrentTask, m.Prompt)
	renderedPrompt := promptStyle.Width(m.width - 2).Render(promptContent)
	m.promptHeight = lipgloss.Height(renderedPrompt)

	history := m.History.View()
	m.historyHeight = 0
	if history != "" {
		m.historyHeight = lipgloss.Height(history)
	}

	footerHeight := lipgloss.Height(m.helpView())

	extraLines := 3
	if m.historyHeight > 0 {
		extraLines = 5
	}
	occupied := m.headerHeight + m.promptHeight + m.historyHeight + footerHeight + extraLines

	vHeight := 20
	if m.expanded {
		vHeight = m.height - occupied
	}

	if occupied+vHeight > m.height {
		vHeight = m.height - occupied
	}
	if vHeight < 2 {
		vHeight = 2
	}

	m.Output.SetSize(m.width, vHeight)
}

func (m TUIModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	header := headerStyle.Render(fmt.Sprintf("Ponder Worker | Model: %s | Iteration: %d/%d", m.ModelName, m.Iterations, m.MaxIterations))

	promptContent := fmt.Sprintf("Task: %s\n\n%s", m.CurrentTask, m.Prompt)
	prompt := promptStyle.Width(m.width - 2).Render(promptContent)

	history := m.History.View()
	if history != "" {
		return fmt.Sprintf("%s\n%s\n\n%s\n%s\n%s",
			header,
			history,
			prompt,
			m.Output.View(),
			m.helpView(),
		)
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s",
		header,
		prompt,
		m.Output.View(),
		m.helpView(),
	)
}

func (m TUIModel) helpView() string {
	help := "Press 'q' to quit • 'e'/'enter' to "
	if m.expanded {
		help += "contract"
	} else {
		help += "expand"
	}
	return statusStyle.Render(help)
}
