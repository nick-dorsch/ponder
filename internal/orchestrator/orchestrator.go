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

type TaskStore interface {
	ClaimNextTask(ctx context.Context) (*models.Task, error)
	UpdateTaskStatus(ctx context.Context, id string, status models.TaskStatus, summary *string) error
	CountAvailableTasks(ctx context.Context) (int, error)
	ResetInProgressTasks(ctx context.Context) error
	DisableOnChange()
	EnableOnChange()
}

type workerInstance struct {
	id       int
	task     *models.Task
	cancel   context.CancelFunc
	done     chan struct{}
	messages chan tea.Msg
}

type failedTaskInfo struct {
	taskID    string
	failedAt  time.Time
	failCount int
}

// Orchestrator manages concurrent task processing.
type Orchestrator struct {
	store           TaskStore
	maxWorkers      int
	targetWorkers   int
	model           string
	availableModels []string
	modelMu         sync.RWMutex
	workers         map[int]*workerInstance
	workersMu       sync.RWMutex
	targetWorkersMu sync.RWMutex
	cmdFactory      func(ctx context.Context, name string, arg ...string) *exec.Cmd
	totalTasks      int
	completedTasks  int
	msgChan         chan tea.Msg
	ctx             context.Context
	cancel          context.CancelFunc
	WebURL          string

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
		targetWorkers:    maxWorkers,
		model:            model,
		availableModels:  []string{model},
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

func (o *Orchestrator) Start(ctx context.Context) error {
	if err := o.store.ResetInProgressTasks(ctx); err != nil {
		o.sendMsg(StatusMsg{WorkerID: 0, Message: fmt.Sprintf("Error resetting in_progress tasks: %v", err)})
	}

	o.ctx, o.cancel = context.WithCancel(ctx)
	defer o.cancel()
	defer close(o.msgChan)

	spawnTicker := time.NewTicker(100 * time.Millisecond)
	defer spawnTicker.Stop()

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
				return nil
			}
		}
	}
}

func (o *Orchestrator) setIdle(idle bool) {
	o.idleMu.Lock()
	defer o.idleMu.Unlock()

	if o.isIdle != idle {
		o.isIdle = idle
		o.sendMsg(IdleStateMsg{Idle: idle})
	}
}

func (o *Orchestrator) Stop() {
	if o.cancel != nil {
		o.cancel()
	}
}

func (o *Orchestrator) Messages() <-chan tea.Msg {
	return o.msgChan
}

// trySpawnWorkers attempts to spawn new workers up to the concurrency limit.
func (o *Orchestrator) trySpawnWorkers() {
	if !o.canSpawn() {
		return
	}

	o.workersMu.Lock()
	activeWorkers := len(o.workers)
	o.workersMu.Unlock()

	targetWorkers := o.GetTargetWorkers()
	if targetWorkers <= activeWorkers {
		return
	}

	if activeWorkers >= o.maxWorkers {
		return
	}

	availableCtx, cancel := context.WithTimeout(o.ctx, 2*time.Second)
	availableCount, err := o.store.CountAvailableTasks(availableCtx)
	cancel()

	if err != nil {
		o.sendMsg(StatusMsg{WorkerID: 0, Message: fmt.Sprintf("Error counting available tasks: %v", err)})
		return
	}

	if availableCount == 0 {
		return
	}

	workersToSpawn := availableCount
	if workersToSpawn > targetWorkers-activeWorkers {
		workersToSpawn = targetWorkers - activeWorkers
	}
	if workersToSpawn > o.maxWorkers-activeWorkers {
		workersToSpawn = o.maxWorkers - activeWorkers
	}

	for i := 0; i < workersToSpawn; i++ {
		if !o.canSpawn() {
			return
		}

		claimCtx, cancel := context.WithTimeout(o.ctx, 5*time.Second)
		task, err := o.store.ClaimNextTask(claimCtx)
		cancel()

		if err != nil {
			o.sendMsg(StatusMsg{WorkerID: 0, Message: fmt.Sprintf("Error claiming task: %v", err)})
			return
		}

		if task == nil {
			return
		}

		if o.isTaskInBackoff(task.ID) {
			resetCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			o.store.UpdateTaskStatus(resetCtx, task.ID, models.TaskStatusPending, nil)
			cancel()
			continue
		}

		o.updateSpawnTime()

		o.workersMu.Lock()
		o.spawnWorkerLocked(task)
		o.workersMu.Unlock()
	}
}

func (o *Orchestrator) canSpawn() bool {
	o.spawnMu.Lock()
	defer o.spawnMu.Unlock()

	if o.lastSpawnTime.IsZero() {
		return true
	}

	return time.Since(o.lastSpawnTime) >= o.minSpawnInterval
}

func (o *Orchestrator) updateSpawnTime() {
	o.spawnMu.Lock()
	defer o.spawnMu.Unlock()
	o.lastSpawnTime = time.Now()
}

func (o *Orchestrator) isTaskInBackoff(taskID string) bool {
	o.failedTasksMu.RLock()
	defer o.failedTasksMu.RUnlock()

	info, exists := o.failedTasks[taskID]
	if !exists {
		return false
	}

	return time.Since(info.failedAt) < o.backoffDuration
}

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

func (o *Orchestrator) spawnWorkerLocked(task *models.Task) {
	workerID := -1
	for i := 1; i <= o.maxWorkers; i++ {
		if _, busy := o.workers[i]; !busy {
			workerID = i
			break
		}
	}

	if workerID == -1 {
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

	o.sendMsg(WorkerStartedMsg{
		WorkerID: workerID,
		Task:     task,
	})

	go o.runWorker(workerCtx, worker)
}

func (o *Orchestrator) runWorker(ctx context.Context, worker *workerInstance) {
	defer close(worker.done)

	task := worker.task

	o.sendMsg(TaskStartedMsg{
		WorkerID: worker.id,
		TaskName: task.Name,
	})

	prompt := o.constructPrompt(task)
	cmd := o.cmdFactory(ctx, "opencode", "run", "--model", o.GetModel())
	cmd.Stdin = strings.NewReader(prompt)

	output := &outputCapture{
		orchestrator: o,
		workerID:     worker.id,
	}
	cmd.Stdout = output
	cmd.Stderr = output

	err := cmd.Run()
	success := err == nil

	if err != nil {
		o.sendMsg(OutputMsg{
			WorkerID: worker.id,
			Output:   fmt.Sprintf("\n--- Error: %v ---\n", err),
		})

		o.recordTaskFailure(task.ID)

		resetCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if resetErr := o.store.UpdateTaskStatus(resetCtx, task.ID, models.TaskStatusPending, nil); resetErr != nil {
			o.sendMsg(StatusMsg{
				WorkerID: worker.id,
				Message:  fmt.Sprintf("Failed to reset task %s: %v", task.Name, resetErr),
			})
		}
		cancel()
	}

	o.sendMsg(TaskCompletedMsg{
		WorkerID: worker.id,
		TaskName: task.Name,
		Success:  success,
	})

	o.workersMu.Lock()
	delete(o.workers, worker.id)
	if success {
		o.completedTasks++
	}
	o.workersMu.Unlock()
}

func (o *Orchestrator) stopAllWorkers() {
	o.workersMu.Lock()
	workersCopy := make([]*workerInstance, 0, len(o.workers))
	for _, worker := range o.workers {
		workersCopy = append(workersCopy, worker)
	}
	o.workersMu.Unlock()

	for _, worker := range workersCopy {
		worker.cancel()
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, worker := range workersCopy {
		wg.Add(1)
		go func(w *workerInstance) {
			defer wg.Done()
			select {
			case <-w.done:
			case <-timeoutCtx.Done():
			}
		}(worker)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-timeoutCtx.Done():
	}

	o.store.DisableOnChange()
	defer o.store.EnableOnChange()

	o.workersMu.Lock()
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cleanupCancel()

	for id, worker := range o.workers {
		_ = o.store.UpdateTaskStatus(cleanupCtx, worker.task.ID, models.TaskStatusPending, nil)
		delete(o.workers, id)
	}
	o.workersMu.Unlock()
}

func (o *Orchestrator) allWorkersIdle() bool {
	o.workersMu.RLock()
	defer o.workersMu.RUnlock()
	return len(o.workers) == 0
}

func (o *Orchestrator) hasMoreTasks() bool {
	ctx, cancel := context.WithTimeout(o.ctx, 2*time.Second)
	defer cancel()

	count, err := o.store.CountAvailableTasks(ctx)
	if err != nil {
		return false
	}

	return count > 0
}

func (o *Orchestrator) GetActiveWorkers() map[int]*workerInstance {
	o.workersMu.RLock()
	defer o.workersMu.RUnlock()

	result := make(map[int]*workerInstance, len(o.workers))
	for k, v := range o.workers {
		result[k] = v
	}
	return result
}

func (o *Orchestrator) GetStats() (total, completed int) {
	o.workersMu.RLock()
	defer o.workersMu.RUnlock()
	return o.totalTasks, o.completedTasks
}

func (o *Orchestrator) GetTargetWorkers() int {
	o.targetWorkersMu.RLock()
	defer o.targetWorkersMu.RUnlock()
	return o.targetWorkers
}

func (o *Orchestrator) GetModel() string {
	o.modelMu.RLock()
	defer o.modelMu.RUnlock()
	return o.model
}

func (o *Orchestrator) SetModel(model string) {
	if model == "" {
		return
	}

	o.modelMu.Lock()
	o.model = model
	o.modelMu.Unlock()
}

func (o *Orchestrator) GetAvailableModels() []string {
	o.modelMu.RLock()
	defer o.modelMu.RUnlock()

	models := make([]string, len(o.availableModels))
	copy(models, o.availableModels)
	return models
}

func (o *Orchestrator) SetAvailableModels(models []string) {
	filtered := make([]string, 0, len(models))
	seen := make(map[string]bool, len(models))
	for _, model := range models {
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		filtered = append(filtered, model)
	}

	o.modelMu.Lock()
	defer o.modelMu.Unlock()

	if len(filtered) == 0 {
		filtered = []string{o.model}
	}

	if !seen[o.model] {
		filtered = append(filtered, o.model)
	}

	o.availableModels = filtered
}

func (o *Orchestrator) SetTargetWorkers(target int) {
	if target < 0 {
		target = 0
	}
	if target > o.maxWorkers {
		target = o.maxWorkers
	}

	o.targetWorkersMu.Lock()
	o.targetWorkers = target
	o.targetWorkersMu.Unlock()
}

func (o *Orchestrator) IncreaseWorkers() bool {
	o.targetWorkersMu.Lock()
	defer o.targetWorkersMu.Unlock()

	if o.targetWorkers >= o.maxWorkers {
		return false
	}
	o.targetWorkers++
	return true
}

func (o *Orchestrator) DecreaseWorkersIfIdle() bool {
	o.workersMu.RLock()
	activeWorkers := len(o.workers)
	o.workersMu.RUnlock()

	o.targetWorkersMu.Lock()
	defer o.targetWorkersMu.Unlock()

	if o.targetWorkers <= 0 {
		return false
	}
	if o.targetWorkers <= activeWorkers {
		return false
	}

	o.targetWorkers--
	return true
}

func (o *Orchestrator) sendMsg(msg tea.Msg) {
	select {
	case o.msgChan <- msg:
	case <-time.After(100 * time.Millisecond):
	}
}

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
