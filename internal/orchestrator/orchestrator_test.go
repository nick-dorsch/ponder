package orchestrator

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nick-dorsch/ponder/embed/prompts"
	"github.com/nick-dorsch/ponder/pkg/models"
)

// mockTaskStore is a mock implementation of TaskStore for testing.
type mockTaskStore struct {
	mu            sync.Mutex
	tasks         []*models.Task
	claimed       map[string]bool
	statusUpdates []statusUpdate
	errors        map[string]error
	nextTaskIndex int

	onChangeDisabled bool
	disableCalled    bool
	enableCalled     bool
}

type statusUpdate struct {
	id     string
	status models.TaskStatus
}

func newMockTaskStore() *mockTaskStore {
	return &mockTaskStore{
		claimed:       make(map[string]bool),
		statusUpdates: make([]statusUpdate, 0),
		errors:        make(map[string]error),
	}
}

func (m *mockTaskStore) ClaimNextTask(ctx context.Context) (*models.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.nextTaskIndex >= len(m.tasks) {
		return nil, nil
	}

	task := m.tasks[m.nextTaskIndex]
	m.nextTaskIndex++
	m.claimed[task.ID] = true

	// Update status to in_progress
	task.Status = models.TaskStatusInProgress
	m.statusUpdates = append(m.statusUpdates, statusUpdate{id: task.ID, status: models.TaskStatusInProgress})

	return task, nil
}

func (m *mockTaskStore) UpdateTaskStatus(ctx context.Context, id string, status models.TaskStatus, summary *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err, ok := m.errors[id]; ok {
		return err
	}

	m.statusUpdates = append(m.statusUpdates, statusUpdate{id: id, status: status})

	// Update the task status in the list
	for _, task := range m.tasks {
		if task.ID == id {
			task.Status = status
			break
		}
	}

	return nil
}

func (m *mockTaskStore) CountAvailableTasks(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, task := range m.tasks {
		if task.Status == models.TaskStatusPending {
			count++
		}
	}
	return count, nil
}

func (m *mockTaskStore) ResetInProgressTasks(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, task := range m.tasks {
		if task.Status == models.TaskStatusInProgress {
			task.Status = models.TaskStatusPending
		}
	}
	return nil
}

func (m *mockTaskStore) DisableOnChange() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChangeDisabled = true
	m.disableCalled = true
}

func (m *mockTaskStore) EnableOnChange() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChangeDisabled = false
	m.enableCalled = true
}

func (m *mockTaskStore) addTask(id, name string, priority int) *models.Task {
	task := &models.Task{
		ID:            id,
		Name:          name,
		Description:   "Test description",
		Specification: "Test specification",
		Priority:      priority,
		Status:        models.TaskStatusPending,
	}
	m.tasks = append(m.tasks, task)
	return task
}

func TestNewOrchestrator(t *testing.T) {
	store := newMockTaskStore()

	// Test with valid parameters
	o := NewOrchestrator(store, 3, "test-model")
	if o.maxWorkers != 3 {
		t.Errorf("expected maxWorkers=3, got %d", o.maxWorkers)
	}
	if o.model != "test-model" {
		t.Errorf("expected model='test-model', got %s", o.model)
	}

	// Test with zero maxWorkers (should default to 3)
	o = NewOrchestrator(store, 0, "")
	if o.maxWorkers != 3 {
		t.Errorf("expected maxWorkers=3 (default), got %d", o.maxWorkers)
	}
	if o.model != "opencode/gemini-3-flash" {
		t.Errorf("expected default model, got %s", o.model)
	}
}

func TestOrchestrator_SingleWorker(t *testing.T) {
	store := newMockTaskStore()
	store.addTask("1", "task1", 1)

	o := NewOrchestrator(store, 1, "test-model")
	// Mock command factory to simulate success
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := o.Start(ctx)
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify task was claimed
	if !store.claimed["1"] {
		t.Error("expected task to be claimed")
	}

	// Verify status was updated to in_progress then pending (since we reset on completion for now)
	// Actually, since we're mocking with "true", it should complete successfully
	if len(store.statusUpdates) < 1 {
		t.Errorf("expected at least 1 status update, got %d", len(store.statusUpdates))
	}
}

func TestOrchestrator_MultipleWorkers(t *testing.T) {
	store := newMockTaskStore()
	store.addTask("1", "task1", 3)
	store.addTask("2", "task2", 2)
	store.addTask("3", "task3", 1)

	o := NewOrchestrator(store, 3, "test-model")
	// Mock command factory to simulate success with a small delay
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := o.Start(ctx)
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all tasks were claimed
	for _, id := range []string{"1", "2", "3"} {
		if !store.claimed[id] {
			t.Errorf("expected task %s to be claimed", id)
		}
	}

	// Verify correct number of status updates (at least 3 for in_progress)
	inProgressCount := 0
	for _, update := range store.statusUpdates {
		if update.status == models.TaskStatusInProgress {
			inProgressCount++
		}
	}
	if inProgressCount != 3 {
		t.Errorf("expected 3 in_progress updates, got %d", inProgressCount)
	}
}

func TestOrchestrator_ConcurrencyLimit(t *testing.T) {
	store := newMockTaskStore()
	store.addTask("1", "task1", 1)
	store.addTask("2", "task2", 1)
	store.addTask("3", "task3", 1)
	store.addTask("4", "task4", 1)

	// Set maxWorkers to 2
	o := NewOrchestrator(store, 2, "test-model")
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := o.Start(ctx)
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatalf("unexpected error: %v", err)
	}

	// All tasks should be completed
	total, completed := o.GetStats()
	if total != 4 {
		t.Errorf("expected total=4, got %d", total)
	}
	if completed != 4 {
		t.Errorf("expected completed=4, got %d", completed)
	}
}

func TestOrchestrator_TaskFailureReset(t *testing.T) {
	store := newMockTaskStore()
	store.addTask("1", "task1", 1)

	o := NewOrchestrator(store, 1, "test-model")
	// Mock command factory to simulate failure
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := o.Start(ctx)
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify task was claimed
	if !store.claimed["1"] {
		t.Error("expected task to be claimed")
	}

	// Verify task was reset to pending after failure
	store.mu.Lock()
	var pendingCount int
	for _, update := range store.statusUpdates {
		if update.status == models.TaskStatusPending {
			pendingCount++
		}
	}
	store.mu.Unlock()

	if pendingCount < 1 {
		t.Errorf("expected at least 1 pending update (reset after failure), got %d", pendingCount)
	}
}

func TestOrchestrator_Stop(t *testing.T) {
	store := newMockTaskStore()
	store.addTask("1", "task1", 1)

	o := NewOrchestrator(store, 1, "test-model")
	// Use a command that respects context cancellation
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		// This command blocks until context is cancelled
		return exec.CommandContext(ctx, "sh", "-c", "while true; do sleep 0.1; done")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start orchestrator in a goroutine
	var startErr error
	var startDone = make(chan struct{})
	go func() {
		startErr = o.Start(ctx)
		close(startDone)
	}()

	// Let it start a worker
	time.Sleep(200 * time.Millisecond)

	// Stop the orchestrator
	o.Stop()

	// Wait for Start to return
	select {
	case <-startDone:
		// Good
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for Start to return")
	}

	if startErr != nil && startErr != context.Canceled {
		t.Errorf("expected context.Canceled or nil, got %v", startErr)
	}
}

func TestOrchestrator_GracefulShutdownReset(t *testing.T) {
	store := newMockTaskStore()
	store.addTask("1", "task1", 1)
	store.addTask("2", "task2", 1)

	o := NewOrchestrator(store, 2, "test-model")
	o.minSpawnInterval = 0 // Disable rate limiting for test
	// Use a command that blocks so workers are active when we stop
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "10")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start orchestrator in a goroutine
	startDone := make(chan struct{})
	go func() {
		_ = o.Start(ctx)
		close(startDone)
	}()

	// Let it start workers
	time.Sleep(200 * time.Millisecond)

	// Verify workers are active
	active := o.GetActiveWorkers()
	if len(active) == 0 {
		t.Fatal("expected active workers")
	}

	// Stop the orchestrator
	o.Stop()

	// Wait for Start to return
	select {
	case <-startDone:
		// Good
	case <-time.After(7 * time.Second):
		t.Fatal("timeout waiting for Start to return")
	}

	// Verify tasks were reset to pending
	store.mu.Lock()
	resetMap := make(map[string]bool)
	for _, update := range store.statusUpdates {
		if update.status == models.TaskStatusPending {
			resetMap[update.id] = true
		}
	}
	store.mu.Unlock()

	if !resetMap["1"] {
		t.Error("expected task 1 to be reset to pending")
	}
	if !resetMap["2"] {
		t.Error("expected task 2 to be reset to pending")
	}
}

func TestOrchestrator_Messages(t *testing.T) {
	store := newMockTaskStore()
	store.addTask("1", "task1", 1)

	o := NewOrchestrator(store, 1, "test-model")
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "test output")
	}

	// Collect messages
	var messages []interface{}
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a goroutine to collect messages
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case msg := <-o.Messages():
				mu.Lock()
				messages = append(messages, msg)
				mu.Unlock()
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
				return
			}
		}
	}()

	err := o.Start(ctx)
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatalf("unexpected error: %v", err)
	}

	<-done

	mu.Lock()
	msgCount := len(messages)
	mu.Unlock()

	if msgCount == 0 {
		t.Error("expected to receive messages")
	}

	// Check for expected message types
	var hasWorkerStarted, hasTaskStarted, hasCompleted bool
	mu.Lock()
	for _, msg := range messages {
		switch msg.(type) {
		case WorkerStartedMsg:
			hasWorkerStarted = true
		case TaskStartedMsg:
			hasTaskStarted = true
		case OutputMsg:
			// Output is optional
		case TaskCompletedMsg:
			hasCompleted = true
		}
	}
	mu.Unlock()

	if !hasWorkerStarted {
		t.Error("expected WorkerStartedMsg")
	}
	if !hasTaskStarted {
		t.Error("expected TaskStartedMsg")
	}
	if !hasCompleted {
		t.Error("expected TaskCompletedMsg")
	}
}

func TestOrchestrator_ChannelClosure(t *testing.T) {
	store := newMockTaskStore()
	o := NewOrchestrator(store, 1, "test-model")
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := o.Start(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}

	// Channel should be closed
	select {
	case _, ok := <-o.Messages():
		if ok {
			t.Error("expected Messages() channel to be closed")
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for channel closure")
	}
}

func TestOrchestrator_GetActiveWorkers(t *testing.T) {
	store := newMockTaskStore()
	store.addTask("1", "task1", 1)
	store.addTask("2", "task2", 1)

	o := NewOrchestrator(store, 2, "test-model")
	// Use a command that takes time so workers stay active
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "0.5")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check active workers during execution
	go func() {
		time.Sleep(100 * time.Millisecond)
		active := o.GetActiveWorkers()
		if len(active) == 0 {
			t.Log("No active workers found (they might have completed quickly)")
		}
	}()

	err := o.Start(ctx)
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatalf("unexpected error: %v", err)
	}

	// After completion, should have no active workers
	active := o.GetActiveWorkers()
	if len(active) != 0 {
		t.Errorf("expected 0 active workers after completion, got %d", len(active))
	}
}

func TestOrchestrator_GetStats(t *testing.T) {
	store := newMockTaskStore()
	store.addTask("1", "task1", 1)
	store.addTask("2", "task2", 1)

	o := NewOrchestrator(store, 2, "test-model")
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := o.Start(ctx)
	if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		t.Fatalf("unexpected error: %v", err)
	}

	total, completed := o.GetStats()
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if completed != 2 {
		t.Errorf("expected completed=2, got %d", completed)
	}
}

func TestOrchestrator_NoTasks(t *testing.T) {
	store := newMockTaskStore()
	// No tasks added

	o := NewOrchestrator(store, 3, "test-model")
	o.PollingInterval = 0 // Ensure it exits

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err := o.Start(ctx)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("expected no error with no tasks, got %v", err)
	}

	// Should return quickly when no tasks
	if duration > 1*time.Second {
		t.Errorf("expected quick return with no tasks, took %v", duration)
	}
}

func TestOrchestrator_Polling(t *testing.T) {
	store := newMockTaskStore()
	// No tasks initially

	o := NewOrchestrator(store, 1, "test-model")
	o.PollingInterval = 100 * time.Millisecond
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "true")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start orchestrator
	errChan := make(chan error, 1)
	go func() {
		errChan <- o.Start(ctx)
	}()

	// Wait a bit to ensure it's polling
	time.Sleep(200 * time.Millisecond)

	// Check if it's idle
	o.idleMu.Lock()
	isIdle := o.isIdle
	o.idleMu.Unlock()
	if !isIdle {
		t.Error("expected orchestrator to be idle")
	}

	// Add a task now
	store.addTask("1", "task1", 1)

	// Wait for orchestrator to pick it up
	time.Sleep(500 * time.Millisecond)

	// Verify task was claimed
	if !store.claimed["1"] {
		t.Error("expected task to be claimed while polling")
	}

	// Stop orchestrator
	cancel()
	err := <-errChan
	if err != nil && err != context.Canceled {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOrchestrator_ShutdownOptimization(t *testing.T) {
	store := newMockTaskStore()
	store.addTask("1", "task1", 1)

	o := NewOrchestrator(store, 1, "test-model")
	// Use a command that blocks
	o.cmdFactory = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "10")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startDone := make(chan struct{})
	go func() {
		_ = o.Start(ctx)
		close(startDone)
	}()

	// Let it start worker
	time.Sleep(200 * time.Millisecond)

	// Stop the orchestrator
	o.Stop()

	// Wait for Start to return
	select {
	case <-startDone:
		// Good
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for Start to return")
	}

	// Verify DisableOnChange was called
	store.mu.Lock()
	disabled := store.disableCalled
	resetDone := false
	for _, update := range store.statusUpdates {
		if update.id == "1" && update.status == models.TaskStatusPending {
			resetDone = true
			break
		}
	}
	store.mu.Unlock()

	if !disabled {
		t.Error("expected DisableOnChange to be called during shutdown")
	}
	if !resetDone {
		t.Error("expected task to be reset to pending during shutdown")
	}
}

func TestConstructPrompt(t *testing.T) {
	o := &Orchestrator{}
	task := &models.Task{
		FeatureName:   "test-feature",
		Name:          "test-task",
		Description:   "test-desc",
		Specification: "test-spec",
	}

	prompt := o.constructPrompt(task)

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
