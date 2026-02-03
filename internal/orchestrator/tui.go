package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nick-dorsch/ponder/internal/ui/components"
)

var (
	orbStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")). // Ponder cyan #22d3ee
			Bold(true)

	headerTextStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Padding(0, 1)

	completedHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("42")).
				Padding(0, 1)

	statsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	headerStyle = lipgloss.NewStyle().
			Padding(1, 2)
)

type OrchestratorModel struct {
	orchestrator   *Orchestrator
	workerViews    map[int]*WorkerView
	completedTasks *components.CompletedTasks
	focusedWorker  int
	width          int
	height         int
	ready          bool
	quitting       bool
	isIdle         bool
	err            error
	workerOrder    []int
	scrollOffset   int
	sidebarWidth   int
	workersWidth   int
}

func NewOrchestratorModel(orch *Orchestrator) *OrchestratorModel {
	comp := components.NewCompletedTasks(0)
	comp.Title = "Completed Tasks"

	workerViews := make(map[int]*WorkerView)
	workerOrder := make([]int, 0)

	for i := 1; i <= orch.maxWorkers; i++ {
		view := NewWorkerView(i, 80, 6)
		workerViews[i] = view
		workerOrder = append(workerOrder, i)
	}

	m := &OrchestratorModel{
		orchestrator:   orch,
		workerViews:    workerViews,
		completedTasks: comp,
		workerOrder:    workerOrder,
		focusedWorker:  1,
	}

	if len(workerOrder) > 0 {
		workerViews[1].SetFocused(true)
	}

	return m
}

func (m *OrchestratorModel) Init() tea.Cmd {
	return tea.Batch(
		m.pollMessages(),
	)
}

func (m *OrchestratorModel) pollMessages() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.orchestrator.Messages()
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *OrchestratorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.orchestrator.Stop()
			return m, tea.Quit
		case "j", "down":
			if !m.isAnyWorkerExpanded() {
				m.moveFocus(1)
			}
		case "k", "up":
			if !m.isAnyWorkerExpanded() {
				m.moveFocus(-1)
			}
		case "e", "enter":
			m.toggleExpanded()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.recalculateLayout()

	case WorkerStartedMsg:
	case TaskCompletedMsg:
		m.completedTasks.Add(components.TaskResult{
			Name:    msg.TaskName,
			Success: msg.Success,
		}, 100)

	case IdleStateMsg:
		m.isIdle = msg.Idle

	case error:
		m.err = msg
		return m, tea.Quit
	}

	for _, view := range m.workerViews {
		if cmd := view.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	switch msg.(type) {
	case WorkerStartedMsg, TaskStartedMsg, OutputMsg, StatusMsg, TaskCompletedMsg, IdleStateMsg, error:
		cmds = append(cmds, m.pollMessages())
	}

	return m, tea.Batch(cmds...)
}

func (m *OrchestratorModel) moveFocus(direction int) {
	if len(m.workerOrder) == 0 {
		return
	}

	currentIdx := -1
	for i, id := range m.workerOrder {
		if id == m.focusedWorker {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		m.focusedWorker = m.workerOrder[0]
	} else {
		newIdx := currentIdx + direction
		if newIdx < 0 {
			newIdx = len(m.workerOrder) - 1
		} else if newIdx >= len(m.workerOrder) {
			newIdx = 0
		}
		m.focusedWorker = m.workerOrder[newIdx]
	}

	for id, view := range m.workerViews {
		view.SetFocused(id == m.focusedWorker)
	}

	m.scrollIntoView()
}

func (m *OrchestratorModel) scrollIntoView() {
	if len(m.workerOrder) == 0 {
		return
	}

	headerHeight := m.getHeaderHeight()
	helpHeight := 1
	availableHeight := m.height - headerHeight - helpHeight
	if availableHeight <= 0 {
		return
	}

	focusedIdx := -1
	for i, id := range m.workerOrder {
		if id == m.focusedWorker {
			focusedIdx = i
			break
		}
	}

	if focusedIdx == -1 {
		return
	}

	topPos := 0
	for i := 0; i < focusedIdx; i++ {
		topPos += m.workerViews[m.workerOrder[i]].GetHeight()
	}
	workerHeight := m.workerViews[m.focusedWorker].GetHeight()
	bottomPos := topPos + workerHeight

	if topPos < m.scrollOffset {
		m.scrollOffset = topPos
	} else if bottomPos > m.scrollOffset+availableHeight {
		m.scrollOffset = bottomPos - availableHeight
	}
}

func (m *OrchestratorModel) toggleExpanded() {
	if view, ok := m.workerViews[m.focusedWorker]; ok {
		expanded := !view.IsExpanded()
		if expanded {
			for _, v := range m.workerViews {
				v.SetExpanded(false)
			}
		}
		view.SetExpanded(expanded)
		m.recalculateLayout()
		m.scrollIntoView()
	}
}

func (m *OrchestratorModel) isAnyWorkerExpanded() bool {
	for _, v := range m.workerViews {
		if v.IsExpanded() {
			return true
		}
	}
	return false
}

func (m *OrchestratorModel) recalculateLayout() {
	if !m.ready {
		return
	}

	m.sidebarWidth = m.width / 5
	if m.sidebarWidth < 20 {
		m.sidebarWidth = 20
	}
	m.workersWidth = m.width - m.sidebarWidth

	headerHeight := m.getHeaderHeight()
	helpHeight := 1
	availableHeight := m.height - headerHeight - helpHeight

	if availableHeight < 10 {
		availableHeight = 10
	}

	m.completedTasks.Width = m.sidebarWidth - 1

	for _, view := range m.workerViews {
		if view.IsExpanded() {
			view.SetSize(m.workersWidth-2, availableHeight)
		} else {
			view.SetSize(m.workersWidth-2, 15)
		}
	}
}

func (m *OrchestratorModel) getHeaderHeight() int {
	header := m.renderHeader()
	return lipgloss.Height(header)
}

func (m *OrchestratorModel) renderCompletedTasks() string {
	return m.completedTasks.View()
}

func (m *OrchestratorModel) View() string {
	if !m.ready {
		return "Initializing orchestrator..."
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	header := m.renderHeader()
	headerHeight := lipgloss.Height(header)

	help := m.renderHelp()
	helpHeight := lipgloss.Height(help)

	availableHeight := m.height - headerHeight - helpHeight
	if availableHeight < 0 {
		availableHeight = 0
	}

	sidebarContent := m.renderCompletedTasks()
	sidebar := lipgloss.NewStyle().
		Width(m.sidebarWidth-1).
		Height(availableHeight).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color("240")).
		Render(sidebarContent)

	var workerList strings.Builder
	for _, workerID := range m.workerOrder {
		if view, ok := m.workerViews[workerID]; ok {
			workerList.WriteString(view.View())
			workerList.WriteString("\n")
		}
	}

	workerContent := workerList.String()
	lines := strings.Split(workerContent, "\n")

	startLine := m.scrollOffset
	if startLine < 0 {
		startLine = 0
	}

	endLine := startLine + availableHeight
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > endLine {
		startLine = endLine
	}

	clippedWorkers := strings.Join(lines[startLine:endLine], "\n")

	clippedHeight := lipgloss.Height(clippedWorkers)
	if clippedHeight < availableHeight {
		clippedWorkers += strings.Repeat("\n", availableHeight-clippedHeight)
	}

	workersArea := lipgloss.NewStyle().
		Width(m.workersWidth).
		Height(availableHeight).
		Render(clippedWorkers)

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, workersArea)

	return header + "\n" + mainContent + "\n" + help
}

func (m *OrchestratorModel) renderHeader() string {
	total, completed := m.orchestrator.GetStats()
	status := "Active"
	if m.isIdle {
		status = "Waiting for tasks..."
	}

	headerText := fmt.Sprintf("Ponder Orchestrator | %s | Model: %s | Workers: %d/%d | Tasks: %d/%d",
		status,
		m.orchestrator.model,
		len(m.orchestrator.GetActiveWorkers()),
		m.orchestrator.maxWorkers,
		completed,
		total,
	)
	if m.orchestrator.WebURL != "" {
		headerText += fmt.Sprintf(" | Web UI: %s", m.orchestrator.WebURL)
	}

	orb := orbStyle.Render("⬤")
	text := headerTextStyle.Render(headerText)

	header := lipgloss.JoinHorizontal(lipgloss.Center, orb, "  ", text)
	return headerStyle.Copy().Width(m.width - 4).Render(header)
}

func (m *OrchestratorModel) renderHelp() string {
	help := "Press 'q' to quit • 'j'/'k' to navigate • 'e'/'enter' to expand/collapse"
	return helpStyle.Render(help)
}

func Run(ctx context.Context, orchestrator *Orchestrator) error {
	m := NewOrchestratorModel(orchestrator)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	orchDone := make(chan struct{})
	var orchErr error

	go func() {
		<-ctx.Done()
		orchestrator.Stop()
	}()

	go func() {
		defer close(orchDone)
		orchErr = orchestrator.Start(context.Background())
		time.Sleep(100 * time.Millisecond)
		p.Quit()
	}()

	_, err := p.Run()

	orchestrator.Stop()
	<-orchDone

	if orchErr != nil && orchErr != context.Canceled {
		return orchErr
	}
	return err
}
