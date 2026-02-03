package worker

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nick-dorsch/ponder/embed/prompts"
	"github.com/nick-dorsch/ponder/pkg/models"
)

type mockStore struct {
	tasks       []*models.Task
	err         error
	calls       int
	statusCalls int
	lastStatus  models.TaskStatus
}

func (m *mockStore) GetAvailableTasks(ctx context.Context) ([]*models.Task, error) {
	m.calls++
	return m.tasks, m.err
}

func (m *mockStore) UpdateTaskStatus(ctx context.Context, id string, status models.TaskStatus, summary *string) error {
	m.statusCalls++
	m.lastStatus = status
	return nil
}

func TestNewWorker(t *testing.T) {
	mock := &mockStore{}
	interval := 10 * time.Second
	model := "test-model"
	maxIter := 100

	w := NewWorker(mock, interval, model, maxIter)

	if w.interval != interval {
		t.Errorf("expected interval %v, got %v", interval, w.interval)
	}
	if w.model != model {
		t.Errorf("expected model %v, got %v", model, w.model)
	}
	if w.maxIterations != maxIter {
		t.Errorf("expected maxIterations %v, got %v", maxIter, w.maxIterations)
	}
}

func TestWorker_Iterations(t *testing.T) {
	mock := &mockStore{
		tasks: []*models.Task{
			{Name: "task1", Priority: 1, FeatureName: "feat1"},
		},
	}

	w := NewWorker(mock, 1*time.Millisecond, "mock-model", 2)
	w.NoTUI = true

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := w.Run(ctx)
	if err != nil && err != context.DeadlineExceeded {
		if !strings.Contains(err.Error(), "executable file not found") && !strings.Contains(err.Error(), "opencode failed") {
			t.Logf("Expected opencode to fail or not be found, got: %v", err)
		}
	}

	if mock.calls == 0 {
		t.Error("expected at least one call to GetAvailableTasks")
	}
	if mock.statusCalls == 0 {
		t.Error("expected at least one call to UpdateTaskStatus")
	}
}

func TestWorker_ResetOnFailure(t *testing.T) {
	mock := &mockStore{
		tasks: []*models.Task{
			{ID: "1", Name: "task1", Priority: 1, FeatureName: "feat1"},
		},
	}

	w := NewWorker(mock, 1*time.Millisecond, "mock-model", 1)
	w.NoTUI = true
	w.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "ls", "/non-existent-directory-ponder-test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	w.Run(ctx)

	if mock.statusCalls < 2 {
		t.Errorf("expected at least 2 status updates (in_progress, then pending), got %d", mock.statusCalls)
	}

	if mock.lastStatus != models.TaskStatusPending {
		t.Errorf("expected final status to be %s, got %s", models.TaskStatusPending, mock.lastStatus)
	}
}

func TestConstructPrompt(t *testing.T) {
	w := &Worker{}
	task := &models.Task{
		FeatureName:   "test-feature",
		Name:          "test-task",
		Description:   "test-desc",
		Specification: "test-spec",
	}

	prompt := w.constructPrompt(task)

	if !strings.HasPrefix(prompt, prompts.Header) {
		t.Error("prompt does not start with Header")
	}
	if !strings.HasSuffix(prompt, prompts.Footer) {
		t.Error("prompt does not end with Footer")
	}
	if !strings.Contains(prompt, "# Feature: test-feature") {
		t.Error("prompt missing feature name")
	}
	if !strings.Contains(prompt, "# Task: test-task") {
		t.Error("prompt missing task name")
	}
	if !strings.Contains(prompt, "## Description\ntest-desc") {
		t.Error("prompt missing description")
	}
	if !strings.Contains(prompt, "## Specification\ntest-spec") {
		t.Error("prompt missing specification")
	}
}

func TestWorker_IterationCount(t *testing.T) {
	mock := &mockStore{
		tasks: []*models.Task{
			{ID: "1", Name: "task1"},
			{ID: "2", Name: "task2"},
			{ID: "3", Name: "task3"},
			{ID: "4", Name: "task4"},
		},
	}

	maxIter := 3
	w := NewWorker(mock, 1*time.Millisecond, "mock-model", maxIter)
	w.NoTUI = true

	w.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := w.Run(ctx)
	if err != nil {
		t.Fatalf("Worker failed: %v", err)
	}

	if mock.statusCalls != maxIter {
		t.Errorf("Expected exactly %d task status updates, got %d", maxIter, mock.statusCalls)
	}
}
