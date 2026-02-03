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

func NewWorkerView(workerID int, width int, height int) *WorkerView {
	return &WorkerView{
		WorkerID: workerID,
		width:    width,
		height:   height,
		Output:   components.NewWorkerOutput(width, height),
		Status:   "idle",
	}
}

func (w *WorkerView) SetSize(width, height int) {
	w.width = width
	w.height = height
	w.ready = true
	w.updateOutputSize()
}

func (w *WorkerView) SetFocused(focused bool) {
	w.focused = focused
}

func (w *WorkerView) SetExpanded(expanded bool) {
	w.expanded = expanded
	w.updateOutputSize()
}

func (w *WorkerView) updateOutputSize() {
	headerHeight := lipgloss.Height(w.renderHeader())
	underlineHeight := lipgloss.Height(w.renderUnderline())
	if w.expanded {
		w.Output.SetSize(w.width-4, w.height-headerHeight-underlineHeight-4)
	} else {
		w.Output.SetSize(w.width-4, 6)
	}
}

func (w *WorkerView) StartTask(taskName string) {
	w.TaskName = taskName
	w.Status = "running"
	w.Output.Reset()
}

func (w *WorkerView) Reset() {
	w.TaskName = ""
	w.Status = "idle"
	w.Output.Reset()
}

func (w *WorkerView) CompleteTask(success bool) {
	if success {
		w.Status = "completed"
	} else {
		w.Status = "failed"
	}
}

func (w *WorkerView) AppendOutput(output string) {
	w.Output.Append(output)
}

func (w *WorkerView) GetHeight() int {
	if w.expanded {
		return w.height
	}
	headerHeight := lipgloss.Height(w.renderHeader())
	underlineHeight := lipgloss.Height(w.renderUnderline())
	return headerHeight + 1 + underlineHeight + 1 + w.Output.Height() + 2
}

func (w *WorkerView) renderHeader() string {
	statusStr := w.getStatusString()
	header := fmt.Sprintf("Worker %d: %s [%s]", w.WorkerID, w.TaskName, statusStr)

	width := w.width - 4
	if width < 0 {
		width = 0
	}

	return workerHeaderStyle.Copy().
		Width(width).
		Render(header)
}

func (w *WorkerView) renderUnderline() string {
	width := w.width - 4
	if width < 0 {
		width = 0
	}

	contentWidth := width - 2
	if contentWidth < 0 {
		contentWidth = 0
	}

	underline := strings.Repeat("─", contentWidth)

	return workerUnderlineStyle.Copy().
		Width(width).
		Render(underline)
}

func (w *WorkerView) View() string {
	if !w.ready {
		return "Initializing..."
	}

	borderStyle := workerActiveStyle
	if w.focused {
		borderStyle = workerFocusedStyle
	}

	headerRendered := w.renderHeader()
	underlineRendered := w.renderUnderline()

	content := fmt.Sprintf("%s\n%s\n%s", headerRendered, underlineRendered, w.Output.View())

	return borderStyle.
		Width(w.width).
		Height(w.GetHeight() - 2).
		Render(content)
}

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
	}

	return w.Output.Update(msg)
}

func (w *WorkerView) GetWorkerID() int {
	return w.WorkerID
}

func (w *WorkerView) GetTaskName() string {
	return w.TaskName
}

func (w *WorkerView) IsRunning() bool {
	return w.Status == "running"
}

func (w *WorkerView) IsExpanded() bool {
	return w.expanded
}

func (w *WorkerView) IsFocused() bool {
	return w.focused
}
