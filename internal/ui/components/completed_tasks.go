package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	completedTaskStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42")).
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("42")).
				Padding(0, 1)

	failedTaskStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("196")).
			Padding(0, 1)

	completedHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("252")).
				Padding(0, 1)

	subTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	placeholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Italic(true).
				Padding(0, 1)
)

// TaskResult represents the result of a completed task.
type TaskResult struct {
	Name    string
	Success bool
}

// CompletedTasks is a component that renders a list of completed tasks.
type CompletedTasks struct {
	Succeeded []TaskResult
	Failed    []TaskResult
	Width     int
	Title     string

	// Compatibility fields
	History       []TaskResult
	SuccessStatus string
	FailureStatus string
	Compact       bool
}

// NewCompletedTasks creates a new CompletedTasks component.
func NewCompletedTasks(width int) *CompletedTasks {
	return &CompletedTasks{
		Succeeded:     make([]TaskResult, 0),
		Failed:        make([]TaskResult, 0),
		Width:         width,
		Title:         "Completed Tasks",
		SuccessStatus: "COMPLETED",
		FailureStatus: "FAILED",
		Compact:       true,
	}
}

// Add adds a task result to the history, keeping only the last n results per list.
func (c *CompletedTasks) Add(res TaskResult, limit int) {
	if res.Success {
		c.Succeeded = append(c.Succeeded, res)
		if limit > 0 && len(c.Succeeded) > limit {
			c.Succeeded = c.Succeeded[len(c.Succeeded)-limit:]
		}
	} else {
		c.Failed = append(c.Failed, res)
		if limit > 0 && len(c.Failed) > limit {
			c.Failed = c.Failed[len(c.Failed)-limit:]
		}
	}

	// Maintain History for compatibility
	c.History = append(c.History, res)
	if limit > 0 && len(c.History) > limit {
		c.History = c.History[len(c.History)-limit:]
	}
}

// View renders the completed tasks.
func (c *CompletedTasks) View() string {
	var boxes []string

	if len(c.Succeeded) > 0 {
		boxes = append(boxes, c.renderBox("Succeeded", c.Succeeded, completedTaskStyle, "✓"))
	}

	if len(c.Failed) > 0 {
		boxes = append(boxes, c.renderBox("Failed", c.Failed, failedTaskStyle, "✗"))
	}

	var content string
	if len(boxes) == 0 {
		content = placeholderStyle.Render("No completed tasks yet")
	} else {
		content = strings.Join(boxes, "\n")
	}

	result := content
	if c.Title != "" {
		result = completedHeaderStyle.Render(c.Title) + "\n" + content
	}
	return result
}

func (c *CompletedTasks) renderBox(title string, tasks []TaskResult, style lipgloss.Style, icon string) string {
	boxWidth := c.Width

	// Sub-title
	subTitle := subTitleStyle.Foreground(style.GetForeground()).Render(title)

	// Width for content (minus borders and padding)
	// NormalBorder takes 1 col on each side = 2
	// Padding takes 1 col on each side = 2
	innerWidth := boxWidth - 4
	if innerWidth < 0 {
		innerWidth = 0
	}

	// Word-level wrapping for tasks
	var lines []string
	nameWidth := innerWidth - 2
	if nameWidth < 0 {
		nameWidth = 0
	}

	for _, t := range tasks {
		wrappedName := lipgloss.NewStyle().Width(nameWidth).Render(t.Name)
		nameLines := strings.Split(wrappedName, "\n")
		for i, line := range nameLines {
			if i == 0 {
				lines = append(lines, fmt.Sprintf("%s %s", icon, line))
			} else {
				lines = append(lines, fmt.Sprintf("  %s", line))
			}
		}
	}

	body := strings.Join(lines, "\n")
	return style.Width(boxWidth).Render(subTitle + "\n" + body)
}
