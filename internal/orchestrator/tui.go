package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ldi/ponder/internal/ui/components"
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

// OrchestratorModel is the Bubble Tea model for the orchestrator TUI.
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
	// IDs of workers in display order
	workerOrder  []int
	scrollOffset int
	sidebarWidth int
	workersWidth int
}

// NewOrchestratorModel creates a new TUI model for the orchestrator.
func NewOrchestratorModel(orch *Orchestrator) *OrchestratorModel {
	comp := components.NewCompletedTasks(0)
	comp.Title = "Completed Tasks"
	comp.SuccessStatus = "✓"
	comp.FailureStatus = "✗"
	comp.Compact = false

	workerViews := make(map[int]*WorkerView)
	workerOrder := make([]int, 0)

	for i := 1; i <= orch.maxWorkers; i++ {
		view := NewWorkerView(i, 80, 6) // Default width, will be updated on WindowSizeMsg
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

// Init initializes the TUI model.
func (m *OrchestratorModel) Init() tea.Cmd {
	// Start a ticker to poll for messages from the orchestrator
	return tea.Batch(
		m.pollMessages(),
	)
}

// pollMessages returns a command that polls for orchestrator messages.
func (m *OrchestratorModel) pollMessages() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.orchestrator.Messages()
		if !ok {
			return nil
		}
		return msg
	}
}

// Update handles messages and updates the model.
func (m *OrchestratorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		// Mouse scrolling disabled

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
		// Worker started, we might want to focus it or just let it be
		// m.focusedWorker = msg.WorkerID
		// m.scrollIntoView()

	case TaskCompletedMsg:
		// Add to completed tasks
		m.completedTasks.Add(components.TaskResult{
			Name:    msg.TaskName,
			Success: msg.Success,
		}, 100) // Increase limit for sidebar

	case IdleStateMsg:
		m.isIdle = msg.Idle

	case error:
		m.err = msg
		return m, tea.Quit

	case nil:
		// No message, just continue
	}

	// Forward messages to worker views
	for _, view := range m.workerViews {
		if cmd := view.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Continue polling if it was an orchestrator message
	switch msg.(type) {
	case WorkerStartedMsg, TaskStartedMsg, OutputMsg, StatusMsg, TaskCompletedMsg, IdleStateMsg, error:
		cmds = append(cmds, m.pollMessages())
	}

	return m, tea.Batch(cmds...)
}

// moveFocus moves the focus to the next/previous worker.
func (m *OrchestratorModel) moveFocus(direction int) {
	if len(m.workerOrder) == 0 {
		return
	}

	// Find current index
	currentIdx := -1
	for i, id := range m.workerOrder {
		if id == m.focusedWorker {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		// Current focus not found, focus first
		m.focusedWorker = m.workerOrder[0]
	} else {
		// Move in direction
		newIdx := currentIdx + direction
		if newIdx < 0 {
			newIdx = len(m.workerOrder) - 1
		} else if newIdx >= len(m.workerOrder) {
			newIdx = 0
		}
		m.focusedWorker = m.workerOrder[newIdx]
	}

	// Update focus on views
	for id, view := range m.workerViews {
		view.SetFocused(id == m.focusedWorker)
	}

	// Ensure focused worker is visible
	m.scrollIntoView()
}

// scrollIntoView ensures the focused worker is visible in the scrollable area.
func (m *OrchestratorModel) scrollIntoView() {
	if len(m.workerOrder) == 0 {
		return
	}

	headerHeight := m.getHeaderHeight()
	helpHeight := 1
	availableHeight := m.height - headerHeight - helpHeight - 2
	if availableHeight <= 0 {
		return
	}

	// Find focused index
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

	// Calculate top position of focused worker relative to start of worker list
	topPos := 0
	for i := 0; i < focusedIdx; i++ {
		topPos += m.workerViews[m.workerOrder[i]].GetHeight()
	}
	workerHeight := m.workerViews[m.focusedWorker].GetHeight()
	bottomPos := topPos + workerHeight

	// Adjust scroll offset
	if topPos < m.scrollOffset {
		m.scrollOffset = topPos
	} else if bottomPos > m.scrollOffset+availableHeight {
		m.scrollOffset = bottomPos - availableHeight
	}
}

// toggleExpanded toggles the expanded state of the focused worker.
func (m *OrchestratorModel) toggleExpanded() {
	if view, ok := m.workerViews[m.focusedWorker]; ok {
		expanded := !view.IsExpanded()
		// If expanding, collapse others
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

// isAnyWorkerExpanded returns true if any worker view is expanded.
func (m *OrchestratorModel) isAnyWorkerExpanded() bool {
	for _, v := range m.workerViews {
		if v.IsExpanded() {
			return true
		}
	}
	return false
}

// recalculateLayout recalculates the layout for all worker views.
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
	// Available height for content: total height - header - help - separator line
	availableHeight := m.height - headerHeight - helpHeight - 2

	if availableHeight < 10 {
		availableHeight = 10
	}

	// Update completed tasks width
	// sidebarWidth is total including right border, so content is sidebarWidth-1.
	// CompletedTasks box has 2-char border overhead.
	m.completedTasks.Width = m.sidebarWidth - 1 - 2

	for _, view := range m.workerViews {
		if view.IsExpanded() {
			view.SetSize(m.workersWidth-2, availableHeight)
		} else {
			// Collapsed workers handle their own height in updateOutputSize,
			// but we pass a constrained width.
			view.SetSize(m.workersWidth-2, 15)
		}
	}
}

// getHeaderHeight returns the height of the header.
func (m *OrchestratorModel) getHeaderHeight() int {
	header := m.renderHeader()
	return lipgloss.Height(header)
}

// renderCompletedTasks renders the completed tasks section.
func (m *OrchestratorModel) renderCompletedTasks() string {
	return m.completedTasks.View()
}

// View renders the TUI.
func (m *OrchestratorModel) View() string {
	if !m.ready {
		return "Initializing orchestrator..."
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	// 1. Header (Full width)
	header := m.renderHeader()
	headerHeight := lipgloss.Height(header)

	// 2. Help (Full width at bottom)
	help := m.renderHelp()
	helpHeight := lipgloss.Height(help)

	// 3. Main Content Area (Two Columns)
	availableHeight := m.height - headerHeight - helpHeight - 2
	if availableHeight < 0 {
		availableHeight = 0
	}

	// Left Column: Completed Tasks Sidebar
	sidebarContent := m.renderCompletedTasks()
	sidebar := lipgloss.NewStyle().
		Width(m.sidebarWidth-1).
		Height(availableHeight).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color("240")).
		Render(sidebarContent)

	// Right Column: Scrollable Workers
	var workerList strings.Builder
	for _, workerID := range m.workerOrder {
		if view, ok := m.workerViews[workerID]; ok {
			workerList.WriteString(view.View())
			workerList.WriteString("\n")
		}
	}

	// Clip worker list based on scroll offset
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

	// Pad clipped workers to fill available height
	clippedHeight := lipgloss.Height(clippedWorkers)
	if clippedHeight < availableHeight {
		clippedWorkers += strings.Repeat("\n", availableHeight-clippedHeight)
	}

	workersArea := lipgloss.NewStyle().
		Width(m.workersWidth).
		Height(availableHeight).
		Render(clippedWorkers)

	// Join Columns
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, workersArea)

	return header + "\n" + mainContent + "\n" + help
}

// renderHeader renders the orchestrator header.
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

// renderHelp renders the help footer.
func (m *OrchestratorModel) renderHelp() string {
	help := "Press 'q' to quit • 'j'/'k' to navigate • 'e'/'enter' to expand/collapse"
	return helpStyle.Render(help)
}

// Run starts the orchestrator TUI.
func Run(ctx context.Context, orchestrator *Orchestrator) error {
	m := NewOrchestratorModel(orchestrator)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	orchDone := make(chan struct{})
	var orchErr error

	// Handle graceful shutdown from the context (e.g. SIGINT)
	go func() {
		<-ctx.Done()
		orchestrator.Stop()
	}()

	// Start orchestrator in a goroutine
	go func() {
		defer close(orchDone)
		// Use context.Background() because we handle cancellation via orchestrator.Stop()
		orchErr = orchestrator.Start(context.Background())
		// Give a moment for final messages to be processed
		time.Sleep(100 * time.Millisecond)
		p.Quit()
	}()

	// Run the TUI
	_, err := p.Run()

	// Ensure orchestrator is stopped if TUI exited first (e.g. error or other quit)
	orchestrator.Stop()

	// Wait for orchestrator to finish cleanup
	<-orchDone

	if orchErr != nil && orchErr != context.Canceled {
		return orchErr
	}
	return err
}
