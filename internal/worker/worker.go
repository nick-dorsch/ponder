package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ldi/ponder/embed/prompts"
	"github.com/ldi/ponder/pkg/models"
)

// TaskStore defines the interface for database operations required by the worker.
type TaskStore interface {
	GetAvailableTasks(ctx context.Context) ([]*models.Task, error)
	UpdateTaskStatus(ctx context.Context, id string, status models.TaskStatus, summary *string) error
}

// Worker represents the background task processor.
type Worker struct {
	store         TaskStore
	interval      time.Duration
	model         string
	maxIterations int
	program       *tea.Program
	NoTUI         bool
	cmdFactory    func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

// NewWorker creates a new Worker instance.
func NewWorker(store TaskStore, interval time.Duration, model string, maxIterations int) *Worker {
	if interval == 0 {
		interval = 5 * time.Second
	}
	return &Worker{
		store:         store,
		interval:      interval,
		model:         model,
		maxIterations: maxIterations,
		cmdFactory:    exec.CommandContext,
	}
}

// Run starts the orchestration loop.
func (w *Worker) Run(ctx context.Context) error {
	if w.NoTUI {
		return w.workerLoop(ctx)
	}

	m := NewTUIModel(w.model, w.maxIterations)
	w.program = tea.NewProgram(m, tea.WithMouseCellMotion())

	done := make(chan struct{})
	var loopErr error

	go func() {
		defer close(done)
		loopErr = w.workerLoop(ctx)
		if loopErr != nil && loopErr != context.Canceled {
			w.program.Send(loopErr)
		}
		w.program.Quit()
	}()

	_, err := w.program.Run()
	<-done

	if loopErr != nil && loopErr != context.Canceled {
		return loopErr
	}
	return err
}

func (w *Worker) workerLoop(ctx context.Context) error {
	iterations := 1
	for {
		if w.maxIterations > 0 && iterations > w.maxIterations {
			w.sendStatus(fmt.Sprintf("Reached max iterations (%d), stopping...", w.maxIterations))
			return nil
		}

		select {
		case <-ctx.Done():
			w.sendStatus("Worker stopping...")
			return ctx.Err()
		default:
			w.sendIteration(iterations)
			processed, task, err := w.processNextTask(ctx)
			if err != nil {
				w.sendOutput(fmt.Sprintf("Error processing task: %v\n", err))
				if task != nil {
					w.sendTaskResult(task.Name, false)
					// Reset task to pending on failure so it can be retried.
					// Use a fresh context because the loop context might be canceled.
					w.sendStatus(fmt.Sprintf("Resetting task %s to pending...", task.Name))
					resetCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					if resetErr := w.store.UpdateTaskStatus(resetCtx, task.ID, models.TaskStatusPending, nil); resetErr != nil {
						w.sendOutput(fmt.Sprintf("Warning: failed to reset task %s to pending: %v\n", task.Name, resetErr))
					}
					cancel()
				}
			}

			if processed {
				w.sendTaskResult(task.Name, true)
				iterations++
				// Continue to next task immediately
				continue
			}

			w.sendStatus("No tasks available, work complete.")
			return nil
		}
	}
}

func (w *Worker) sendStatus(msg string) {
	if w.program != nil {
		w.program.Send(StatusMsg(msg))
	} else {
		fmt.Printf("--- %s ---\n", msg)
	}
}

func (w *Worker) sendIteration(i int) {
	if w.program != nil {
		w.program.Send(IterationMsg(i))
	}
}

func (w *Worker) sendTaskResult(name string, success bool) {
	if w.program != nil {
		w.program.Send(TaskResultMsg{Name: name, Success: success})
	}
}

func (w *Worker) sendOutput(msg string) {
	if w.program != nil {
		w.program.Send(OutputMsg(msg))
	} else {
		fmt.Print(msg)
	}
}

func (w *Worker) processNextTask(ctx context.Context) (bool, *models.Task, error) {
	tasks, err := w.store.GetAvailableTasks(ctx)
	if err != nil {
		return false, nil, err
	}

	if len(tasks) == 0 {
		return false, nil, nil
	}

	task := tasks[0]
	if w.program != nil {
		w.program.Send(TaskMsg{Name: task.Name, Prompt: task.Description})
	} else {
		fmt.Printf("Processing task: %s\n", task.Name)
	}

	prompt := w.constructPrompt(task)

	// Mark task as in_progress before agent launch
	if err := w.store.UpdateTaskStatus(ctx, task.ID, models.TaskStatusInProgress, nil); err != nil {
		return true, task, fmt.Errorf("failed to set task %s to in_progress: %w", task.Name, err)
	}

	cmd := w.cmdFactory(ctx, "opencode", "run", "--model", w.model)
	cmd.Stdin = strings.NewReader(prompt)

	if w.program != nil {
		writer := &tuiWriter{p: w.program}
		cmd.Stdout = writer
		cmd.Stderr = writer
	} else {
		// Use a simple writer that prints to stdout in NoTUI mode
		// Actually cmd.Stdout = os.Stdout is fine, but let's be consistent
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		return true, task, fmt.Errorf("opencode failed for task %s: %w", task.Name, err)
	}

	return true, task, nil
}

type tuiWriter struct {
	p *tea.Program
}

func (w *tuiWriter) Write(p []byte) (n int, err error) {
	w.p.Send(OutputMsg(string(p)))
	return len(p), nil
}

func (w *Worker) constructPrompt(task *models.Task) string {
	var sb strings.Builder
	sb.WriteString(prompts.Header)
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("# Feature: %s\n# Task: %s\n\n", task.FeatureName, task.Name))
	sb.WriteString(fmt.Sprintf("## Description\n%s\n\n", task.Description))
	sb.WriteString(fmt.Sprintf("## Specification\n%s\n\n", task.Specification))
	sb.WriteString(prompts.Footer)
	return sb.String()
}
