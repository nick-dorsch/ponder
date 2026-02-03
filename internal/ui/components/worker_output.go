package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	outputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	scrollbarTrackStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("236"))

	scrollbarHandleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))
)

// WorkerOutput renders worker output in a viewport.
type WorkerOutput struct {
	viewport viewport.Model
	output   strings.Builder
	ready    bool
	width    int
	height   int
}

// NewWorkerOutput creates a new WorkerOutput.
func NewWorkerOutput(width, height int) *WorkerOutput {
	return &WorkerOutput{
		viewport: viewport.New(width, height),
		width:    width,
		height:   height,
	}
}

func (o *WorkerOutput) SetSize(width, height int) {
	o.width = width
	o.height = height
	vpWidth := width
	if width > 0 {
		vpWidth = width - 1
	}
	if !o.ready {
		o.viewport = viewport.New(vpWidth, height)
		o.viewport.HighPerformanceRendering = false
		o.ready = true
	} else {
		o.viewport.Width = vpWidth
		o.viewport.Height = height
	}
	o.updateContent()
}

func (o *WorkerOutput) Append(content string) {
	o.output.WriteString(content)
	o.updateContent()
}

func (o *WorkerOutput) AppendStatus(status string) {
	o.output.WriteString(statusStyle.Render(fmt.Sprintf("\n--- %s ---\n", status)))
	o.updateContent()
}

func (o *WorkerOutput) SetContent(content string) {
	o.output.Reset()
	o.output.WriteString(content)
	o.updateContent()
}

func (o *WorkerOutput) Reset() {
	o.output.Reset()
	o.updateContent()
}

func (o *WorkerOutput) updateContent() {
	width := o.viewport.Width
	content := o.output.String()
	if width > 0 {
		content = outputStyle.Copy().Width(width).Render(content)
	} else {
		content = outputStyle.Render(content)
	}
	o.viewport.SetContent(content)
	o.viewport.GotoBottom()
}

func (o *WorkerOutput) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	o.viewport, cmd = o.viewport.Update(msg)
	return cmd
}

func (o *WorkerOutput) View() string {
	if !o.ready {
		return ""
	}

	if o.viewport.TotalLineCount() <= o.viewport.Height {
		return o.viewport.View()
	}

	h := o.viewport.Height
	percent := o.viewport.ScrollPercent()

	handlePos := int(float64(h-1) * percent)

	var sb strings.Builder
	for i := 0; i < h; i++ {
		if i == handlePos {
			sb.WriteString(scrollbarHandleStyle.Render("┃"))
		} else {
			sb.WriteString(scrollbarTrackStyle.Render("│"))
		}
		if i < h-1 {
			sb.WriteString("\n")
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, o.viewport.View(), sb.String())
}

func (o *WorkerOutput) GotoBottom() {
	o.viewport.GotoBottom()
}

func (o *WorkerOutput) Height() int {
	return o.viewport.Height
}

func (o *WorkerOutput) SetHeight(height int) {
	o.viewport.Height = height
}
