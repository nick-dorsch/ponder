package orchestrator

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestWorkerFocusedStyle(t *testing.T) {
	w := NewWorkerView(1, 80, 20)
	w.SetSize(80, 20)
	w.SetFocused(true)
	w.StartTask("test-task")

	view := w.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}

	if strings.Contains(view, "226") || strings.Contains(view, "\x1b[38;5;226m") {
		t.Errorf("View still contains old yellow color (226)")
	}
}

func TestWorkerView_Focus(t *testing.T) {
	w := NewWorkerView(1, 80, 20)
	w.SetSize(80, 20)

	w.SetFocused(false)
	viewUnfocused := w.View()

	w.SetFocused(true)
	viewFocused := w.View()

	if viewUnfocused == viewFocused {
		t.Errorf("Focused and unfocused views should be different")
	}
}

func TestWorkerView_IdleState(t *testing.T) {
	w := NewWorkerView(1, 80, 20)
	w.ready = true

	if !strings.Contains(w.getStatusString(), "IDLE") {
		t.Errorf("expected initial status to be IDLE, got %s", w.getStatusString())
	}

	w.StartTask("test-task")
	if !strings.Contains(w.getStatusString(), "RUNNING") {
		t.Errorf("expected status to be RUNNING, got %s", w.getStatusString())
	}

	w.Reset()
	if !strings.Contains(w.getStatusString(), "IDLE") {
		t.Errorf("expected status to be IDLE after reset, got %s", w.getStatusString())
	}
	if w.TaskName != "" {
		t.Errorf("expected TaskName to be empty after reset, got %s", w.TaskName)
	}
}

func TestWorkerView_HeaderWrapping(t *testing.T) {
	width := 20
	w := NewWorkerView(1, width, 20)
	w.ready = true

	longTaskName := "this is a very long task name that should wrap multiple times"
	w.StartTask(longTaskName)

	view := w.View()

	lines := strings.Split(strings.TrimSpace(view), "\n")

	if len(lines) <= 10 {
		t.Errorf("expected height > 10 for long task name, got %d", len(lines))
	}

	h1 := w.GetHeight()
	w.StartTask("short")
	h2 := w.GetHeight()

	if h1 <= h2 {
		t.Errorf("expected height with long name (%d) to be greater than with short name (%d)", h1, h2)
	}
}

func TestWorkerView_RenderUnderline(t *testing.T) {
	w := NewWorkerView(1, 80, 20)
	w.SetSize(80, 20)

	underline := w.renderUnderline()

	if !strings.Contains(underline, "─") {
		t.Error("Underline should contain box-drawing characters")
	}

	expectedWidth := 76
	if lipgloss.Width(underline) != expectedWidth {
		t.Errorf("Expected underline width %d, got %d", expectedWidth, lipgloss.Width(underline))
	}
}

func TestWorkerView_View_ContainsUnderline(t *testing.T) {
	w := NewWorkerView(1, 80, 20)
	w.SetSize(80, 20)
	w.StartTask("test-task")

	view := w.View()

	if !strings.Contains(view, "─") {
		t.Error("View should contain underline")
	}
}

func TestWorkerView_GetHeight_Collapsed(t *testing.T) {
	w := NewWorkerView(1, 80, 20)
	w.SetSize(80, 20)

	height := w.GetHeight()

	expectedHeight := 12
	if height != expectedHeight {
		t.Errorf("Expected collapsed height %d, got %d", expectedHeight, height)
	}
}

func TestWorkerView_ViewLayout(t *testing.T) {
	width := 80
	w := NewWorkerView(1, width, 20)
	w.ready = true
	w.StartTask("test-task")

	view := w.View()

	if !strings.Contains(view, "Worker 1: test-task") {
		t.Errorf("View missing header")
	}
	if !strings.Contains(view, "─") {
		t.Errorf("View missing underline")
	}

	headerIdx := strings.Index(view, "Worker 1: test-task")
	if headerIdx == -1 {
		t.Errorf("View missing header")
	} else {
		underlineIdx := strings.Index(view[headerIdx:], "─")
		if underlineIdx == -1 {
			t.Errorf("Underline not found after header")
		}
	}
}
