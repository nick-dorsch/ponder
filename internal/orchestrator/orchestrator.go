package orchestrator

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nick-dorsch/ponder/embed/prompts"
	"github.com/nick-dorsch/ponder/pkg/models"
)

// TaskStore defines the interface for database operations required by the orchestrator.
type TaskStore interface {
	ClaimNextTask(ctx context.Context) (*models.Task, error)
	UpdateTaskStatus(ctx context.Context, id string, status models.TaskStatus, summary *string) error
	CountAvailableTasks(ctx context.Context) (int, error)
	ResetInProgressTasks(ctx context.Context) error
	DisableOnChange()
	EnableOnChange()
}

// workerInstance represents a single running worker goroutine.
type workerInstance struct {
	id       int
	task     *models.Task
	cancel   context.CancelFunc
	done     chan struct{}
	messages chan tea.Msg
}

// failedTaskInfo tracks information about recently failed tasks
type failedTaskInfo struct {
	taskID    string
	failedAt  time.Time
	failCount int
}

// Orchestrator manages concurrent task processing with multiple workers.
type Orchestrator struct {
	store          TaskStore
	maxWorkers     int
	model          string
	workers        map[int]*workerInstance
	workersMu      sync.RWMutex
	cmdFactory     func(ctx context.Context, name string, arg ...string) *exec.Cmd
	totalTasks     int
	completedTasks int
	msgChan        chan tea.Msg
	ctx            context.Context
	cancel         context.CancelFunc
	WebURL         string

	// Failed task tracking with backoff
	failedTasks     map[string]*failedTaskInfo
	failedTasksMu   sync.RWMutex
	backoffDuration time.Duration

	// Spawn rate limiting
	lastSpawnTime    time.Time
	spawnMu          sync.Mutex
	minSpawnInterval time.Duration

	// Polling state
	PollingInterval time.Duration
	isIdle          bool
	idleMu          sync.Mutex
}

// NewOrchestrator creates a new Orchestrator instance.
func NewOrchestrator(store TaskStore, maxWorkers int, model string) *Orchestrator {
	if maxWorkers <= 0 {
		maxWorkers = 3
	}
	if model == "" {
		model = "opencode/gemini-3-flash"
	}
	return &Orchestrator{
		store:            store,
		maxWorkers:       maxWorkers,
		model:            model,
		workers:          make(map[int]*workerInstance),
		cmdFactory:       exec.CommandContext,
		msgChan:          make(chan tea.Msg, 100),
		failedTasks:      make(map[string]*failedTaskInfo),
		backoffDuration:  30 * time.Second,
		minSpawnInterval: 500 * time.Millisecond,
		lastSpawnTime:    time.Time{},
		PollingInterval:  0,
	}
}

// Start begins the orchestration loop.
func (o *Orchestrator) Start(ctx context.Context) error {
	// Reset orphaned in_progress tasks on startup
	if err := o.store.ResetInProgressTasks(ctx); err != nil {
		o.sendMsg(StatusMsg{WorkerID: 0, Message: fmt.Sprintf("Error resetting in_progress tasks: %v", err)})
	}

	o.ctx, o.cancel = context.WithCancel(ctx)
	defer o.cancel()
	defer close(o.msgChan)

	// Start the main coordination loop
	spawnTicker := time.NewTicker(100 * time.Millisecond)
	defer spawnTicker.Stop()

	// Cleanup ticker for removing old failed task entries
	cleanupTicker := time.NewTicker(30 * time.Second)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			o.stopAllWorkers()
			return o.ctx.Err()
		case <-cleanupTicker.C:
			o.cleanupFailedTasks()
		case <-spawnTicker.C:
			o.trySpawnWorkers()

			idle := o.allWorkersIdle() && !o.hasMoreTasks()
			o.setIdle(idle)

			if idle && o.PollingInterval == 0 {
				// No workers running and no more tasks available
				return nil
			}

			// If idle and polling is enabled, we just keep looping.
			// The spawnTicker will continue to fire, and trySpawnWorkers will check for tasks.
		}
	}
}

// setIdle updates the idle state and sends a message if it changed.
func (o *Orchestrator) setIdle(idle bool) {
	o.idleMu.Lock()
	defer o.idleMu.Unlock()

	if o.isIdle != idle {
		o.isIdle = idle
		o.sendMsg(IdleStateMsg{Idle: idle})
	}
}

// Stop gracefully stops the orchestrator and all workers.
func (o *Orchestrator) Stop() {
	if o.cancel != nil {
		o.cancel()
	}
}

// Messages returns the channel for receiving TUI messages.
func (o *Orchestrator) Messages() <-chan tea.Msg {
	return o.msgChan
}

// trySpawnWorkers attempts to spawn new workers up to the concurrency limit.
// It respects the available task count and rate limiting to prevent rapid spawning loops.
func (o *Orchestrator) trySpawnWorkers() {
	// Check rate limiting - ensure minimum time between spawns
	if !o.canSpawn() {
		return
	}

	o.workersMu.Lock()
	activeWorkers := len(o.workers)
	o.workersMu.Unlock()

	if activeWorkers >= o.maxWorkers {
		return
	}

	// Count available tasks to determine how many workers we actually need
	availableCtx, cancel := context.WithTimeout(o.ctx, 2*time.Second)
	availableCount, err := o.store.CountAvailableTasks(availableCtx)
	cancel()

	if err != nil {
		o.sendMsg(StatusMsg{WorkerID: 0, Message: fmt.Sprintf("Error counting available tasks: %v", err)})
		return
	}

	if availableCount == 0 {
		// No tasks available, nothing to spawn
		return
	}

	// Calculate how many workers we should spawn
	// We want min(availableTasks, maxWorkers - activeWorkers, maxWorkers)
	workersToSpawn := availableCount
	if workersToSpawn > o.maxWorkers-activeWorkers {
		workersToSpawn = o.maxWorkers - activeWorkers
	}

	// Spawn workers
	for i := 0; i < workersToSpawn; i++ {
		// Check if we should spawn (rate limiting)
		if !o.canSpawn() {
			return
		}

		// Use a short timeout for claiming to avoid blocking
		claimCtx, cancel := context.WithTimeout(o.ctx, 5*time.Second)
		task, err := o.store.ClaimNextTask(claimCtx)
		cancel()

		if err != nil {
			o.sendMsg(StatusMsg{WorkerID: 0, Message: fmt.Sprintf("Error claiming task: %v", err)})
			return
		}

		if task == nil {
			// No more tasks available
			return
		}

		// Check if this task is in backoff due to recent failure
		if o.isTaskInBackoff(task.ID) {
			// Reset the task to pending so it can be claimed later
			resetCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			o.store.UpdateTaskStatus(resetCtx, task.ID, models.TaskStatusPending, nil)
			cancel()
			continue
		}

		// Update last spawn time for rate limiting
		o.updateSpawnTime()

		// Spawn a new worker for this task
		o.workersMu.Lock()
		o.spawnWorkerLocked(task)
		o.workersMu.Unlock()
	}
}

// canSpawn checks if enough time has passed since the last spawn (rate limiting).
func (o *Orchestrator) canSpawn() bool {
	o.spawnMu.Lock()
	defer o.spawnMu.Unlock()

	if o.lastSpawnTime.IsZero() {
		return true
	}

	return time.Since(o.lastSpawnTime) >= o.minSpawnInterval
}

// updateSpawnTime updates the last spawn time to now.
func (o *Orchestrator) updateSpawnTime() {
	o.spawnMu.Lock()
	defer o.spawnMu.Unlock()
	o.lastSpawnTime = time.Now()
}

// isTaskInBackoff checks if a task should not be retried due to recent failure.
func (o *Orchestrator) isTaskInBackoff(taskID string) bool {
	o.failedTasksMu.RLock()
	defer o.failedTasksMu.RUnlock()

	info, exists := o.failedTasks[taskID]
	if !exists {
		return false
	}

	// Check if backoff period has elapsed
	return time.Since(info.failedAt) < o.backoffDuration
}

// recordTaskFailure records a task failure for backoff tracking.
func (o *Orchestrator) recordTaskFailure(taskID string) {
	o.failedTasksMu.Lock()
	defer o.failedTasksMu.Unlock()

	info, exists := o.failedTasks[taskID]
	if exists {
		info.failCount++
		info.failedAt = time.Now()
	} else {
		o.failedTasks[taskID] = &failedTaskInfo{
			taskID:    taskID,
			failedAt:  time.Now(),
			failCount: 1,
		}
	}
}

// cleanupFailedTasks removes old failed task entries that are past their backoff period.
func (o *Orchestrator) cleanupFailedTasks() {
	o.failedTasksMu.Lock()
	defer o.failedTasksMu.Unlock()

	now := time.Now()
	for id, info := range o.failedTasks {
		if now.Sub(info.failedAt) > o.backoffDuration*2 {
			delete(o.failedTasks, id)
		}
	}
}

// spawnWorkerLocked spawns a new worker goroutine. Must be called with workersMu held.
func (o *Orchestrator) spawnWorkerLocked(task *models.Task) {
	// Find available slot ID
	workerID := -1
	for i := 1; i <= o.maxWorkers; i++ {
		if _, busy := o.workers[i]; !busy {
			workerID = i
			break
		}
	}

	if workerID == -1 {
		// This should not happen if trySpawnWorkers is correct, but be safe
		return
	}

	workerCtx, cancel := context.WithCancel(o.ctx)
	worker := &workerInstance{
		id:       workerID,
		task:     task,
		cancel:   cancel,
		done:     make(chan struct{}),
		messages: make(chan tea.Msg, 50),
	}

	o.workers[workerID] = worker
	o.totalTasks++

	// Send worker started message
	o.sendMsg(WorkerStartedMsg{
		WorkerID: workerID,
		Task:     task,
	})

	// Start the worker goroutine
	go o.runWorker(workerCtx, worker)
}

// runWorker runs a single task in a worker goroutine.
func (o *Orchestrator) runWorker(ctx context.Context, worker *workerInstance) {
	defer close(worker.done)

	task := worker.task

	// Send task started message
	o.sendMsg(TaskStartedMsg{
		WorkerID: worker.id,
		TaskName: task.Name,
	})

	// Construct the prompt
	prompt := o.constructPrompt(task)

	// Create the command
	cmd := o.cmdFactory(ctx, "opencode", "run", "--model", o.model)
	cmd.Stdin = strings.NewReader(prompt)

	// Create output capture
	output := &outputCapture{
		orchestrator: o,
		workerID:     worker.id,
	}
	cmd.Stdout = output
	cmd.Stderr = output

	// Run the command
	err := cmd.Run()

	// Determine success/failure
	success := err == nil

	if err != nil {
		o.sendMsg(OutputMsg{
			WorkerID: worker.id,
			Output:   fmt.Sprintf("\n--- Error: %v ---\n", err),
		})

		// Record the failure for backoff tracking
		o.recordTaskFailure(task.ID)

		// Reset task to pending on failure
		resetCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if resetErr := o.store.UpdateTaskStatus(resetCtx, task.ID, models.TaskStatusPending, nil); resetErr != nil {
			o.sendMsg(StatusMsg{
				WorkerID: worker.id,
				Message:  fmt.Sprintf("Failed to reset task %s: %v", task.Name, resetErr),
			})
		}
		cancel()
	}

	// Send completion message
	o.sendMsg(TaskCompletedMsg{
		WorkerID: worker.id,
		TaskName: task.Name,
		Success:  success,
	})

	// Remove worker from active list
	o.workersMu.Lock()
	delete(o.workers, worker.id)
	if success {
		o.completedTasks++
	}
	o.workersMu.Unlock()
}

// stopAllWorkers stops all running workers and waits for them to finish with a timeout.
func (o *Orchestrator) stopAllWorkers() {
	// First, get a copy of workers and cancel them
	o.workersMu.Lock()
	workersCopy := make([]*workerInstance, 0, len(o.workers))
	for _, worker := range o.workers {
		workersCopy = append(workersCopy, worker)
	}
	o.workersMu.Unlock()

	// Cancel all workers (outside the lock)
	for _, worker := range workersCopy {
		worker.cancel()
	}

	// Wait for all workers to finish with a timeout
	// Use context.Background() for the timeout because o.ctx is already canceled
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, worker := range workersCopy {
		wg.Add(1)
		go func(w *workerInstance) {
			defer wg.Done()
			select {
			case <-w.done:
				// Worker finished
			case <-timeoutCtx.Done():
				// Timeout reached
			}
		}(worker)
	}

	// Wait for wait group or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers finished within timeout
	case <-timeoutCtx.Done():
		// Timeout reached, some workers might still be running
	}

	// Final cleanup: any tasks still marked as active in our internal state
	// must be reset to pending in the database to ensure they aren't orphaned.
	o.store.DisableOnChange()
	defer o.store.EnableOnChange()

	o.workersMu.Lock()
	// Use context.Background() and a longer timeout for cleanup to ensure it's not
	// cut short by the primary context cancellation.
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cleanupCancel()

	for id, worker := range o.workers {
		// Attempt to reset the task status to pending
		_ = o.store.UpdateTaskStatus(cleanupCtx, worker.task.ID, models.TaskStatusPending, nil)
		delete(o.workers, id)
	}
	o.workersMu.Unlock()
}

// allWorkersIdle checks if all workers have finished.
func (o *Orchestrator) allWorkersIdle() bool {
	o.workersMu.RLock()
	defer o.workersMu.RUnlock()
	return len(o.workers) == 0
}

// hasMoreTasks checks if there are more tasks available.
// This uses CountAvailableTasks for efficiency.
func (o *Orchestrator) hasMoreTasks() bool {
	ctx, cancel := context.WithTimeout(o.ctx, 2*time.Second)
	defer cancel()

	count, err := o.store.CountAvailableTasks(ctx)
	if err != nil {
		return false
	}

	return count > 0
}

// GetActiveWorkers returns the currently active workers.
func (o *Orchestrator) GetActiveWorkers() map[int]*workerInstance {
	o.workersMu.RLock()
	defer o.workersMu.RUnlock()

	// Return a copy
	result := make(map[int]*workerInstance, len(o.workers))
	for k, v := range o.workers {
		result[k] = v
	}
	return result
}

// GetStats returns orchestration statistics.
func (o *Orchestrator) GetStats() (total, completed int) {
	o.workersMu.RLock()
	defer o.workersMu.RUnlock()
	return o.totalTasks, o.completedTasks
}

// sendMsg sends a message to the TUI message channel.
func (o *Orchestrator) sendMsg(msg tea.Msg) {
	select {
	case o.msgChan <- msg:
	case <-time.After(100 * time.Millisecond):
		// Channel full or blocked, drop message
	}
}

// constructPrompt builds the prompt for a task.
func (o *Orchestrator) constructPrompt(task *models.Task) string {
	var sb strings.Builder
	sb.WriteString(prompts.Header)
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("# Feature: %s\n# Task: %s\n\n", task.FeatureName, task.Name))
	sb.WriteString(fmt.Sprintf("## Description\n%s\n\n", task.Description))
	sb.WriteString(fmt.Sprintf("## Specification\n%s\n\n", task.Specification))
	sb.WriteString(prompts.Footer)
	return sb.String()
}

// outputCapture captures output from opencode and sends it to the orchestrator.
type outputCapture struct {
	orchestrator *Orchestrator
	workerID     int
}

func (o *outputCapture) Write(p []byte) (n int, err error) {
	o.orchestrator.sendMsg(OutputMsg{
		WorkerID: o.workerID,
		Output:   string(p),
	})
	return len(p), nil
}

// Message types for TUI communication
type WorkerStartedMsg struct {
	WorkerID int
	Task     *models.Task
}

type TaskStartedMsg struct {
	WorkerID int
	TaskName string
}

type OutputMsg struct {
	WorkerID int
	Output   string
}

type StatusMsg struct {
	WorkerID int
	Message  string
}

type TaskCompletedMsg struct {
	WorkerID int
	TaskName string
	Success  bool
}

type IdleStateMsg struct {
	Idle bool
}
