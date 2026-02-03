package orchestrator

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestNewOrchestratorModel(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 3, "test-model")
	m := NewOrchestratorModel(orch)

	if m.orchestrator != orch {
		t.Errorf("expected orchestrator to be set")
	}
	if len(m.workerViews) != 3 {
		t.Errorf("expected 3 worker views initially, got %d", len(m.workerViews))
	}
	if m.completedTasks == nil {
		t.Errorf("expected completedTasks to be initialized")
	}
}

func TestOrchestratorModel_Update(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 3, "test-model")
	m := NewOrchestratorModel(orch)

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	if m.width != 100 || m.height != 50 || !m.ready {
		t.Errorf("failed to handle WindowSizeMsg")
	}

	m.Update(WorkerStartedMsg{WorkerID: 1})
	if len(m.workerViews) != 3 {
		t.Errorf("expected 3 worker views, got %d", len(m.workerViews))
	}
	if _, ok := m.workerViews[1]; !ok {
		t.Errorf("worker view 1 not found")
	}

	m.Update(TaskCompletedMsg{WorkerID: 1, TaskName: "task1", Success: true})
	if len(m.completedTasks.History) != 1 {
		t.Errorf("expected 1 completed task in history")
	}
	if m.completedTasks.History[0].Name != "task1" {
		t.Errorf("expected task1, got %s", m.completedTasks.History[0].Name)
	}
}

func TestOrchestratorModel_Navigation(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 3, "test-model")
	m := NewOrchestratorModel(orch)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if m.focusedWorker != 1 {
		t.Errorf("expected first worker (1) to be focused initially, got %d", m.focusedWorker)
	}

	m.moveFocus(-1)
	if m.focusedWorker != 3 {
		t.Errorf("expected worker 3 to be focused after moving up (wrap around), got %d", m.focusedWorker)
	}

	m.moveFocus(-1)
	if m.focusedWorker != 2 {
		t.Errorf("expected worker 2 to be focused after moving up, got %d", m.focusedWorker)
	}

	m.moveFocus(-1)
	if m.focusedWorker != 1 {
		t.Errorf("expected worker 1 to be focused after moving up, got %d", m.focusedWorker)
	}

	m.moveFocus(1)
	if m.focusedWorker != 2 {
		t.Errorf("expected worker 2 to be focused after moving down, got %d", m.focusedWorker)
	}
}

func TestOrchestratorModel_Scrolling(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 10, "test-model")
	m := NewOrchestratorModel(orch)

	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	m.focusedWorker = 5
	m.scrollIntoView()

	headerHeight := m.getHeaderHeight()
	helpHeight := lipgloss.Height(m.renderHelp())
	availableHeight := m.height - headerHeight - helpHeight

	workerHeight := m.workerViews[1].GetHeight()
	topPos := workerHeight * 4
	bottomPos := topPos + m.workerViews[5].GetHeight()
	expectedMinOffset := bottomPos - availableHeight

	if m.scrollOffset < expectedMinOffset {
		t.Errorf("expected scrollOffset to be at least %d to show worker 5, got %d", expectedMinOffset, m.scrollOffset)
	}

	m.focusedWorker = 1
	m.scrollIntoView()

	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset to be 0 when worker 1 is focused, got %d", m.scrollOffset)
	}
}

func TestOrchestratorModel_Expansion(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 3, "test-model")
	m := NewOrchestratorModel(orch)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	m.focusedWorker = 1
	for id, view := range m.workerViews {
		view.SetFocused(id == m.focusedWorker)
	}

	m.toggleExpanded()

	if !m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be expanded")
	}
	if m.workerViews[2].IsExpanded() {
		t.Errorf("expected worker 2 to be collapsed")
	}

	headerHeight := m.getHeaderHeight()
	helpHeight := lipgloss.Height(m.renderHelp())
	availableHeight := m.height - headerHeight - helpHeight

	if m.workerViews[1].GetHeight() != availableHeight {
		t.Errorf("expected expanded worker height to be %d, got %d", availableHeight, m.workerViews[1].GetHeight())
	}

	m.toggleExpanded()
	if m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be collapsed after second toggle")
	}
}

func TestOrchestratorModel_View(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 3, "test-model")
	m := NewOrchestratorModel(orch)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
	if !strings.Contains(view, "Ponder Orchestrator") {
		t.Error("View() missing header")
	}
	if !strings.Contains(view, "⬤") {
		t.Error("View() missing orb")
	}
	if !strings.Contains(view, "Worker 1") {
		t.Error("View() missing worker 1")
	}
}

func TestOrchestratorModel_RenderHeader_WithOrb(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 3, "test-model")
	orch.WebURL = "http://localhost:8080"
	m := NewOrchestratorModel(orch)
	m.isIdle = true

	header := m.renderHeader()

	if !strings.Contains(header, "⬤") {
		t.Error("header missing orb character")
	}

	expectedElements := []string{
		"Ponder Orchestrator",
		"Waiting for tasks...",
		"Model: test-model",
		"Workers: 0/3",
		"Tasks: 0/0",
		"Web UI: http://localhost:8080",
	}

	for _, element := range expectedElements {
		if !strings.Contains(header, element) {
			t.Errorf("header missing expected element: %s", element)
		}
	}

	m.isIdle = false
	header = m.renderHeader()
	if !strings.Contains(header, "Active") {
		t.Error("header missing 'Active' status when not idle")
	}
}

func TestOrchestratorModel_GetHeaderHeight(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 3, "test-model")
	m := NewOrchestratorModel(orch)

	height := m.getHeaderHeight()

	if height != 3 {
		t.Errorf("expected header height to be 3, got %d", height)
	}

	orch.WebURL = "http://very-long-url-that-should-not-wrap-unless-width-is-very-small.com"
	height2 := m.getHeaderHeight()
	if height2 != height {
		t.Errorf("expected header height to be consistent, got %d and %d", height, height2)
	}
}

func TestOrchestratorModel_RecalculateLayout_HeaderIntegration(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 1, "test-model")
	m := NewOrchestratorModel(orch)

	width, height := 100, 40
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})

	view := m.workerViews[1]

	m.toggleExpanded()
	headerHeight := m.getHeaderHeight()
	helpHeight := lipgloss.Height(m.renderHelp())
	availableHeight := m.height - headerHeight - helpHeight
	if view.GetHeight() != availableHeight {
		t.Errorf("expected expanded worker height to be %d, got %d", availableHeight, view.GetHeight())
	}
}

func TestWorkerViewHeight(t *testing.T) {
	w := NewWorkerView(1, 80, 12)
	w.SetSize(80, 12)
	w.ready = true
	view := w.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) != 12 {
		t.Errorf("expected WorkerView to be 12 lines, got %d", len(lines))
		for i, line := range lines {
			t.Logf("%d: %s", i, line)
		}
	}
}

func TestOrchestratorModel_ContextAwareScrolling(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 3, "test-model")
	m := NewOrchestratorModel(orch)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if m.focusedWorker != 1 {
		t.Errorf("expected worker 1 to be focused, got %d", m.focusedWorker)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if !m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be expanded")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.focusedWorker != 1 {
		t.Errorf("expected focus to stay on worker 1 when expanded, got %d", m.focusedWorker)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be collapsed")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.focusedWorker != 2 {
		t.Errorf("expected focus to move to worker 2 when collapsed, got %d", m.focusedWorker)
	}
}

func TestOrchestratorModel_EnterKeyExpansion(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 3, "test-model")
	m := NewOrchestratorModel(orch)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	m.focusedWorker = 1
	for id, view := range m.workerViews {
		view.SetFocused(id == m.focusedWorker)
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be expanded via Enter key")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be collapsed via Enter key after second toggle")
	}
}
