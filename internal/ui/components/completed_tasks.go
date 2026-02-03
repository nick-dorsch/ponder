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

type TaskResult struct {
	Name    string
	Success bool
}

type CompletedTasks struct {
	Succeeded []TaskResult
	Failed    []TaskResult
	Width     int
	Title     string

	// Compatibility fields
	History []TaskResult
}

func NewCompletedTasks(width int) *CompletedTasks {
	return &CompletedTasks{
		Succeeded: make([]TaskResult, 0),
		Failed:    make([]TaskResult, 0),
		Width:     width,
		Title:     "Completed Tasks",
	}
}

func (c *CompletedTasks) Add(res TaskResult, limit int) {
	if res.Success {
		c.Succeeded = c.appendWithLimit(c.Succeeded, res, limit)
	} else {
		c.Failed = c.appendWithLimit(c.Failed, res, limit)
	}

	c.History = c.appendWithLimit(c.History, res, limit)
}

func (c *CompletedTasks) appendWithLimit(slice []TaskResult, res TaskResult, limit int) []TaskResult {
	slice = append(slice, res)
	if limit > 0 && len(slice) > limit {
		return slice[len(slice)-limit:]
	}
	return slice
}

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

	subTitle := subTitleStyle.Foreground(style.GetForeground()).Render(title)

	innerWidth := boxWidth - 4
	if innerWidth < 0 {
		innerWidth = 0
	}

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
