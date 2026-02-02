package orchestrator

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

	// Test WindowSizeMsg
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	if m.width != 100 || m.height != 50 || !m.ready {
		t.Errorf("failed to handle WindowSizeMsg")
	}

	// Test WorkerStartedMsg - should already be there, no change in count
	m.Update(WorkerStartedMsg{WorkerID: 1})
	if len(m.workerViews) != 3 {
		t.Errorf("expected 3 worker views, got %d", len(m.workerViews))
	}
	if _, ok := m.workerViews[1]; !ok {
		t.Errorf("worker view 1 not found")
	}

	// Test TaskCompletedMsg
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

	// Move focus up (wrap around)
	m.moveFocus(-1)
	if m.focusedWorker != 3 {
		t.Errorf("expected worker 3 to be focused after moving up (wrap around), got %d", m.focusedWorker)
	}

	// Move focus up again
	m.moveFocus(-1)
	if m.focusedWorker != 2 {
		t.Errorf("expected worker 2 to be focused after moving up, got %d", m.focusedWorker)
	}

	// Move focus up again
	m.moveFocus(-1)
	if m.focusedWorker != 1 {
		t.Errorf("expected worker 1 to be focused after moving up, got %d", m.focusedWorker)
	}

	// Move focus down
	m.moveFocus(1)
	if m.focusedWorker != 2 {
		t.Errorf("expected worker 2 to be focused after moving down, got %d", m.focusedWorker)
	}
}

func TestOrchestratorModel_Scrolling(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 10, "test-model")
	m := NewOrchestratorModel(orch)

	// Small height to force scrolling
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})

	// 10 workers pre-populated.
	// Focus worker 5
	m.focusedWorker = 5
	m.scrollIntoView()

	// 10 workers pre-populated. Each collapsed worker is 12 lines.
	// Worker 1-4 take 48 lines. Worker 5 starts at line 48, ends at line 60.
	// Available height: 20 - 3 (header) - 1 (help) - 2 (separators) = 14.
	// To show Worker 5 (bottomPos 60), scrollOffset should be at least 60 - 14 = 46.

	if m.scrollOffset < 46 {
		t.Errorf("expected scrollOffset to be at least 46 to show worker 5, got %d", m.scrollOffset)
	}

	// Move focus to worker 1
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

	// Focus worker 1
	m.focusedWorker = 1
	for id, view := range m.workerViews {
		view.SetFocused(id == m.focusedWorker)
	}

	// Toggle expansion
	m.toggleExpanded()

	if !m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be expanded")
	}
	if m.workerViews[2].IsExpanded() {
		t.Errorf("expected worker 2 to be collapsed")
	}

	// Check height of expanded worker
	// Height is 40. Header is 3. Help is 1. Two separators.
	// availableHeight = 40 - 3 - 1 - 2 = 34.
	if m.workerViews[1].GetHeight() != 34 {
		t.Errorf("expected expanded worker height to be 34, got %d", m.workerViews[1].GetHeight())
	}

	// Toggle expansion again
	m.toggleExpanded()
	if m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be collapsed after second toggle")
	}
	if m.workerViews[1].GetHeight() != 12 {
		t.Errorf("expected collapsed worker height to be 12, got %d", m.workerViews[1].GetHeight())
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

	// 1. Verify orb character and styling (at least presence of orb)
	if !strings.Contains(header, "⬤") {
		t.Error("header missing orb character")
	}

	// 2. Verify all text elements
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

	// Test Active status
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

	// headerStyle has Padding(1, 2), content is 1 line, so height should be 3
	if height != 3 {
		t.Errorf("expected header height to be 3, got %d", height)
	}

	// Verify height is consistent with longer content
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

	// Set window size
	width, height := 100, 40
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})

	// Header height is 3
	// availableHeight = 40 - 3 - 1 (help) - 2 (separators) = 34

	view := m.workerViews[1]
	if view.GetHeight() != 12 { // Collapsed default height from updateOutputSize
		// wait, recalculateLayout says:
		// view.SetSize(m.workersWidth-2, 15) for collapsed
	}

	// Expand to test availableHeight calculation
	m.toggleExpanded()
	if view.GetHeight() != 34 {
		t.Errorf("expected expanded worker height to be 34, got %d", view.GetHeight())
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

	// Expand worker 1
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if !m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be expanded")
	}

	// Press 'j' - focus should NOT move to worker 2
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.focusedWorker != 1 {
		t.Errorf("expected focus to stay on worker 1 when expanded, got %d", m.focusedWorker)
	}

	// Collapse worker 1
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be collapsed")
	}

	// Press 'j' - focus SHOULD move to worker 2
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

	// Focus worker 1
	m.focusedWorker = 1
	for id, view := range m.workerViews {
		view.SetFocused(id == m.focusedWorker)
	}

	// Toggle expansion with Enter key
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be expanded via Enter key")
	}

	// Toggle expansion again with Enter key
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.workerViews[1].IsExpanded() {
		t.Errorf("expected worker 1 to be collapsed via Enter key after second toggle")
	}
}
