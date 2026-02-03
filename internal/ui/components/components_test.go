package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestCompletedTasks(t *testing.T) {
	c := NewCompletedTasks(80)
	c.Title = "History"

	c.Add(TaskResult{Name: "task1", Success: true}, 5)
	c.Add(TaskResult{Name: "task2", Success: false}, 5)

	view := c.View()

	if !strings.Contains(view, "History") {
		t.Errorf("expected view to contain Title")
	}
	if !strings.Contains(view, "Succeeded") {
		t.Errorf("expected view to contain Succeeded")
	}
	if !strings.Contains(view, "Failed") {
		t.Errorf("expected view to contain Failed")
	}
	if !strings.Contains(view, "✓ task1") {
		t.Errorf("expected view to contain ✓ task1")
	}
	if !strings.Contains(view, "✗ task2") {
		t.Errorf("expected view to contain ✗ task2")
	}
}

func TestCompletedTasksChronologicalOrder(t *testing.T) {
	c := NewCompletedTasks(40)
	c.Add(TaskResult{Name: "oldest", Success: true}, 10)
	c.Add(TaskResult{Name: "middle", Success: true}, 10)
	c.Add(TaskResult{Name: "newest", Success: true}, 10)

	view := c.View()
	oldestIdx := strings.Index(view, "oldest")
	middleIdx := strings.Index(view, "middle")
	newestIdx := strings.Index(view, "newest")

	if oldestIdx == -1 || middleIdx == -1 || newestIdx == -1 {
		t.Errorf("expected all tasks to be present")
	}
	if !(oldestIdx < middleIdx && middleIdx < newestIdx) {
		t.Errorf("expected chronological order (oldest first), got indices: %d, %d, %d", oldestIdx, middleIdx, newestIdx)
	}
}

func TestCompletedTasksEmptyState(t *testing.T) {
	c := NewCompletedTasks(80)
	view := c.View()
	if !strings.Contains(view, "No completed tasks yet") {
		t.Errorf("expected placeholder when no tasks")
	}

	c.Add(TaskResult{Name: "task1", Success: true}, 5)
	view = c.View()
	if !strings.Contains(view, "Succeeded") {
		t.Errorf("expected Succeeded box")
	}
	if strings.Contains(view, "Failed") {
		t.Errorf("expected NO Failed box when empty")
	}
}

func TestCompletedTasksWidth(t *testing.T) {
	width := 20
	c := NewCompletedTasks(width)
	c.Add(TaskResult{Name: "task1", Success: true}, 5)

	view := c.View()
	lines := strings.Split(view, "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		w := lipgloss.Width(line)
		if w > width {
			t.Errorf("line too wide: %d > %d. Line: %q", w, width, line)
		}
	}
}

func TestWorkerOutput(t *testing.T) {
	o := NewWorkerOutput(80, 20)
	o.SetSize(80, 20)

	o.Append("hello")
	o.AppendStatus("running")

	view := o.View()
	if !strings.Contains(view, "hello") {
		t.Errorf("expected view to contain hello")
	}
	if !strings.Contains(view, "--- running ---") {
		t.Errorf("expected view to contain status message")
	}

	o.Reset()
	view = o.View()
	if strings.Contains(view, "hello") {
		t.Errorf("expected view to be cleared after Reset")
	}
}

func TestWorkerOutputScrollbar(t *testing.T) {
	width, height := 20, 5
	o := NewWorkerOutput(width, height)
	o.SetSize(width, height)

	for i := 0; i < 10; i++ {
		o.Append("line\n")
	}

	view := o.View()

	if !strings.Contains(view, "┃") {
		t.Errorf("expected view to contain scrollbar handle '┃'")
	}
	if !strings.Contains(view, "│") {
		t.Errorf("expected view to contain scrollbar track '│'")
	}
}

func TestWorkerOutputNoScrollbar(t *testing.T) {
	width, height := 20, 10
	o := NewWorkerOutput(width, height)
	o.SetSize(width, height)

	o.Append("short content")

	view := o.View()

	if strings.Contains(view, "┃") || strings.Contains(view, "│") {
		t.Errorf("expected view to NOT contain scrollbar when content fits")
	}
}

func TestWorkerOutputWrapping(t *testing.T) {
	width, height := 20, 10
	o := NewWorkerOutput(width, height)
	o.SetSize(width, height)

	o.Append("this is a very long line that should definitely wrap because it exceeds the width of twenty characters")

	view := o.View()

	lines := strings.Split(strings.TrimSpace(view), "\n")
	if len(lines) <= 1 {
		t.Errorf("expected content to wrap into multiple lines, but got %d lines. View: %q", len(lines), view)
	}

	for i, line := range lines {
		w := lipgloss.Width(line)
		if w > width {
			t.Errorf("line %d is too wide: %d > %d. Content: %q", i, w, width, line)
		}
	}
}

func TestWorkerOutputReWrappingOnResize(t *testing.T) {
	width, height := 60, 10
	o := NewWorkerOutput(width, height)
	o.SetSize(width, height)

	content := "this is a moderately long line that should fit in sixty characters but not in twenty"
	o.Append(content)

	view1 := o.View()
	lines1 := strings.Split(strings.TrimSpace(view1), "\n")

	o.SetSize(20, 10)
	lines2 := strings.Split(strings.TrimSpace(o.viewport.View()), "\n")

	if len(lines2) <= len(lines1) {
		t.Errorf("expected more lines after shrinking width: %d <= %d", len(lines2), len(lines1))
	}
}
