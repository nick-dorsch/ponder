package orchestrator

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nick-dorsch/ponder/internal/ui/components"
)

var (
	workerHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("205")).
				Padding(0, 1)

	workerUnderlineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Padding(0, 1)

	workerActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(0, 1)

	workerFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder()).
				BorderForeground(lipgloss.Color("12")).
				Padding(0, 1)

	statusRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("33")).
				Bold(true)

	statusSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42")).
				Bold(true)

	statusFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)
)

// WorkerView represents the UI component for a single worker.
type WorkerView struct {
	WorkerID int
	TaskName string
	Status   string // "running", "completed", "failed"
	Output   *components.WorkerOutput
	width    int
	height   int
	expanded bool
	focused  bool
	ready    bool
}

// NewWorkerView creates a new WorkerView instance.
func NewWorkerView(workerID int, width int, height int) *WorkerView {
	return &WorkerView{
		WorkerID: workerID,
		width:    width,
		height:   height,
		Output:   components.NewWorkerOutput(width, height),
		Status:   "idle",
	}
}

// SetSize updates the size of the worker view.
func (w *WorkerView) SetSize(width, height int) {
	w.width = width
	w.height = height
	w.ready = true
	w.updateOutputSize()
}

// SetFocused sets whether this worker view is focused.
func (w *WorkerView) SetFocused(focused bool) {
	w.focused = focused
}

// SetExpanded sets whether this worker view is expanded.
func (w *WorkerView) SetExpanded(expanded bool) {
	w.expanded = expanded
	w.updateOutputSize()
}

func (w *WorkerView) updateOutputSize() {
	headerHeight := lipgloss.Height(w.renderHeader())
	underlineHeight := lipgloss.Height(w.renderUnderline())
	if w.expanded {
		// When expanded, fill the available space minus header, underline, extra newlines and borders
		w.Output.SetSize(w.width-4, w.height-headerHeight-underlineHeight-4)
	} else {
		// REDUCED: from 11 to 6 (reduction of 5)
		w.Output.SetSize(w.width-4, 6)
	}
}

// StartTask initializes the view for a new task.
func (w *WorkerView) StartTask(taskName string) {
	w.TaskName = taskName
	w.Status = "running"
	w.Output.Reset()
}

// Reset returns the worker view to the idle state.
func (w *WorkerView) Reset() {
	w.TaskName = ""
	w.Status = "idle"
	w.Output.Reset()
}

// CompleteTask marks the task as completed.
func (w *WorkerView) CompleteTask(success bool) {
	if success {
		w.Status = "completed"
	} else {
		w.Status = "failed"
	}
}

// AppendOutput adds output to the worker's log.
func (w *WorkerView) AppendOutput(output string) {
	w.Output.Append(output)
}

// GetHeight returns the current height of the view.
func (w *WorkerView) GetHeight() int {
	if w.expanded {
		return w.height
	}
	headerHeight := lipgloss.Height(w.renderHeader())
	underlineHeight := lipgloss.Height(w.renderUnderline())
	// Collapsed height: header + newline + underline + newline + viewport + borders
	return headerHeight + 1 + underlineHeight + 1 + w.Output.Height() + 2
}

func (w *WorkerView) renderHeader() string {
	statusStr := w.getStatusString()
	header := fmt.Sprintf("Worker %d: %s [%s]", w.WorkerID, w.TaskName, statusStr)

	// Calculate available width for content (width - borders - outer padding)
	width := w.width - 4
	if width < 0 {
		width = 0
	}

	return workerHeaderStyle.Copy().
		Width(width).
		Render(header)
}

func (w *WorkerView) renderUnderline() string {
	// Calculate available width for content (width - borders - outer padding)
	width := w.width - 4
	if width < 0 {
		width = 0
	}

	// Adjust for padding (1 left, 1 right)
	contentWidth := width - 2
	if contentWidth < 0 {
		contentWidth = 0
	}

	// Create underline using box-drawing character
	underline := strings.Repeat("─", contentWidth)

	return workerUnderlineStyle.Copy().
		Width(width).
		Render(underline)
}

// View renders the worker view.
func (w *WorkerView) View() string {
	if !w.ready {
		return "Initializing..."
	}

	// Determine border style based on focus
	borderStyle := workerActiveStyle
	if w.focused {
		borderStyle = workerFocusedStyle
	}

	// Build header and underline
	headerRendered := w.renderHeader()
	underlineRendered := w.renderUnderline()

	// Build content
	// New layout: header + newline + underline + newline + output
	content := fmt.Sprintf("%s\n%s\n%s", headerRendered, underlineRendered, w.Output.View())

	// Apply border and ensure height
	return borderStyle.
		Width(w.width).
		Height(w.GetHeight() - 2).
		Render(content)
}

// getStatusString returns the styled status string.
func (w *WorkerView) getStatusString() string {
	switch w.Status {
	case "running":
		return statusRunningStyle.Render("RUNNING")
	case "completed":
		return statusSuccessStyle.Render("COMPLETED")
	case "failed":
		return statusFailedStyle.Render("FAILED")
	default:
		return "IDLE"
	}
}

// Update handles messages for the worker view.
func (w *WorkerView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case OutputMsg:
		if msg.WorkerID == w.WorkerID {
			w.AppendOutput(msg.Output)
		}
	case TaskStartedMsg:
		if msg.WorkerID == w.WorkerID {
			w.StartTask(msg.TaskName)
		}
	case TaskCompletedMsg:
		if msg.WorkerID == w.WorkerID {
			w.CompleteTask(msg.Success)
			w.Reset()
		}
	case tea.KeyMsg:
		if !w.expanded {
			return nil
		}
		// Fall through to w.Output.Update(msg)
	}

	return w.Output.Update(msg)
}

// GetWorkerID returns the worker ID.
func (w *WorkerView) GetWorkerID() int {
	return w.WorkerID
}

// GetTaskName returns the current task name.
func (w *WorkerView) GetTaskName() string {
	return w.TaskName
}

// IsRunning returns true if the worker is running a task.
func (w *WorkerView) IsRunning() bool {
	return w.Status == "running"
}

// IsExpanded returns true if the worker view is expanded.
func (w *WorkerView) IsExpanded() bool {
	return w.expanded
}

// IsFocused returns true if the worker view is focused.
func (w *WorkerView) IsFocused() bool {
	return w.focused
}
