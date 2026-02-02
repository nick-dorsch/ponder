package worker

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestTUILayout(t *testing.T) {
	m := NewTUIModel("test-model", 10)
	m.width = 80
	m.height = 40
	m.ready = true
	m.CurrentTask = "test-task"
	m.Prompt = "test-prompt"

	m.recalculateLayout()

	if m.Output.Height() != 20 {
		t.Errorf("expected viewport height 20, got %d", m.Output.Height())
	}

	// Test with small terminal height
	m.height = 10
	m.recalculateLayout()

	if m.Output.Height() >= 20 {
		t.Errorf("viewport height should be less than 20 in small terminal, got %d", m.Output.Height())
	}

	occupied := lipgloss.Height(headerStyle.Render("")) +
		lipgloss.Height(promptStyle.Width(m.width-2).Render("Task: test-task\n\ntest-prompt")) +
		lipgloss.Height(m.helpView()) + 3

	expectedVHeight := m.height - occupied
	if expectedVHeight < 2 {
		expectedVHeight = 2
	}

	if m.Output.Height() != expectedVHeight {
		t.Errorf("expected viewport height %d, got %d", expectedVHeight, m.Output.Height())
	}
}

func TestTUIExpansion(t *testing.T) {
	m := NewTUIModel("test-model", 10)
	m.width = 80
	m.height = 40
	m.ready = true
	m.CurrentTask = "test-task"
	m.Prompt = "test-prompt"

	// Initial state should not be expanded
	m.recalculateLayout()
	if m.Output.Height() != 20 {
		t.Errorf("expected initial viewport height 20, got %d", m.Output.Height())
	}

	// Toggle expansion
	m.expanded = true
	m.recalculateLayout()

	header := headerStyle.Render(fmt.Sprintf("Ponder Worker | Model: %s | Iteration: %d/%d", m.ModelName, m.Iterations, m.MaxIterations))
	headerHeight := lipgloss.Height(header)
	promptContent := fmt.Sprintf("Task: %s\n\n%s", m.CurrentTask, m.Prompt)
	renderedPrompt := promptStyle.Width(m.width - 2).Render(promptContent)
	promptHeight := lipgloss.Height(renderedPrompt)
	footerHeight := lipgloss.Height(m.helpView())
	// History is present (at least placeholder)
	occupied := headerHeight + promptHeight + m.historyHeight + footerHeight + 5

	expectedHeight := m.height - occupied
	if m.Output.Height() != expectedHeight {
		t.Errorf("expected expanded viewport height %d, got %d", expectedHeight, m.Output.Height())
	}

	// Contract back
	m.expanded = false
	m.recalculateLayout()
	if m.Output.Height() != 20 {
		t.Errorf("expected contracted viewport height 20, got %d", m.Output.Height())
	}
}

func TestTUIHistory(t *testing.T) {
	m := NewTUIModel("test-model", 10)
	m.width = 80
	m.height = 40
	m.ready = true

	// Add 6 history items, should only keep 5
	for i := 1; i <= 6; i++ {
		m.Update(TaskResultMsg{Name: fmt.Sprintf("task%d", i), Success: true})
	}

	if len(m.History.History) != 5 {
		t.Errorf("expected history length 5, got %d", len(m.History.History))
	}

	if m.History.History[0].Name != "task2" {
		t.Errorf("expected first history item to be task2, got %s", m.History.History[0].Name)
	}

	if m.History.History[4].Name != "task6" {
		t.Errorf("expected last history item to be task6, got %s", m.History.History[4].Name)
	}

	// Verify layout accounts for history
	m.recalculateLayout()
	// Each history box has a border, so it's 3 lines high.
	// 5 boxes * 3 lines = 15 lines.
	// But lipgloss might add more if there are margins (none added by me in styles).
	if m.historyHeight == 0 {
		t.Errorf("historyHeight should not be 0")
	}

	// header (1) + prompt (7) + history (15) + footer (1) + extraLines (5) = 29
	// viewport height = 40 - 29 = 11.
	// However, if occupied+vHeight > height, it will be capped.
	// Wait, recalculateLayout has:
	// if vHeight < 2 { vHeight = 2 }
	// Let's see what we got.
	if m.Output.Height() >= 20 {
		t.Errorf("viewport height should be reduced when history is present, got %d", m.Output.Height())
	}
}

func TestTUIAutoTailing(t *testing.T) {
	m := NewTUIModel("test-model", 10)
	m.width = 80
	m.height = 40
	m.ready = true
	m.Output.SetSize(80, 20)

	// Add enough content to exceed viewport height
	content := ""
	for i := 0; i < 30; i++ {
		content += fmt.Sprintf("line %d\n", i)
	}

	m.Update(OutputMsg(content))

	// Note: We can't easily check AtBottom() because it's internal to viewport in WorkerOutput
	// and we don't expose it. For now, just making sure it doesn't crash.
}

func TestTUI_EnterKeyExpansion(t *testing.T) {
	m := NewTUIModel("test-model", 10)
	m.width = 80
	m.height = 40
	m.ready = true

	if m.expanded {
		t.Fatal("expected initial expanded state to be false")
	}

	// Send Enter key
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.expanded {
		t.Error("expected expanded state to be true after Enter key")
	}

	// Send Enter key again
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.expanded {
		t.Error("expected expanded state to be false after second Enter key")
	}

	// Verify 'e' key still works
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if !m.expanded {
		t.Error("expected expanded state to be true after 'e' key")
	}
}
