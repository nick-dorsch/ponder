package db

import (
	"sync"

	"github.com/nick-dorsch/ponder/pkg/models"
)

type StagedItems struct {
	Features     []*models.Feature
	Tasks        []*models.Task
	Dependencies []*models.Dependency
}

// StagingManager provides thread-safe in-memory storage for staged changes.
type StagingManager struct {
	mu     sync.RWMutex
	staged map[string]*StagedItems
}

func NewStagingManager() *StagingManager {
	return &StagingManager{
		staged: make(map[string]*StagedItems),
	}
}

func (sm *StagingManager) AddFeature(sessionID string, feature *models.Feature) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.staged[sessionID] == nil {
		sm.staged[sessionID] = &StagedItems{
			Features:     []*models.Feature{},
			Tasks:        []*models.Task{},
			Dependencies: []*models.Dependency{},
		}
	}
	sm.staged[sessionID].Features = append(sm.staged[sessionID].Features, feature)
}

func (sm *StagingManager) AddTask(sessionID string, task *models.Task) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.staged[sessionID] == nil {
		sm.staged[sessionID] = &StagedItems{
			Features:     []*models.Feature{},
			Tasks:        []*models.Task{},
			Dependencies: []*models.Dependency{},
		}
	}
	sm.staged[sessionID].Tasks = append(sm.staged[sessionID].Tasks, task)
}

func (sm *StagingManager) AddDependency(sessionID string, dep *models.Dependency) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.staged[sessionID] == nil {
		sm.staged[sessionID] = &StagedItems{
			Features:     []*models.Feature{},
			Tasks:        []*models.Task{},
			Dependencies: []*models.Dependency{},
		}
	}
	sm.staged[sessionID].Dependencies = append(sm.staged[sessionID].Dependencies, dep)
}

func (sm *StagingManager) GetAndClear(sessionID string) *StagedItems {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	items, ok := sm.staged[sessionID]
	if !ok {
		return &StagedItems{
			Features:     []*models.Feature{},
			Tasks:        []*models.Task{},
			Dependencies: []*models.Dependency{},
		}
	}

	delete(sm.staged, sessionID)
	return items
}

func (sm *StagingManager) Peek(sessionID string) *StagedItems {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	items, ok := sm.staged[sessionID]
	if !ok {
		return &StagedItems{
			Features:     []*models.Feature{},
			Tasks:        []*models.Task{},
			Dependencies: []*models.Dependency{},
		}
	}

	return items
}
