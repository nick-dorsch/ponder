package worker

import (
	"context"
	"testing"
	"time"

	"github.com/nick-dorsch/ponder/pkg/models"
)

func TestWorker_ExitOnNoTasks(t *testing.T) {
	mock := &mockStore{
		tasks: []*models.Task{},
	}

	w := NewWorker(mock, 1*time.Second, "mock-model", 10)
	w.NoTUI = true

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := w.Run(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if mock.calls != 1 {
		t.Errorf("expected 1 call to GetAvailableTasks, got %d", mock.calls)
	}
}
