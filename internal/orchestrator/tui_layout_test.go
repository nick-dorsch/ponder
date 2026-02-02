package orchestrator

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestOrchestratorModel_WidthFitting(t *testing.T) {
	store := newMockTaskStore()
	orch := NewOrchestrator(store, 3, "test-model")
	m := NewOrchestratorModel(orch)

	widths := []int{80, 100, 120, 150}
	for _, width := range widths {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 40})

		// Test collapsed
		checkWidth(t, m.View(), width, "collapsed")

		// Test expanded
		m.focusedWorker = 1
		m.toggleExpanded()
		checkWidth(t, m.View(), width, "expanded")
		m.toggleExpanded() // collapse back
	}
}

func checkWidth(t *testing.T, view string, width int, state string) {
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth > width {
			t.Errorf("Width %d (%s): line %d is too wide: %d > %d\nLine content: [%s]", width, state, i, lineWidth, width, line)
		}
	}
}
