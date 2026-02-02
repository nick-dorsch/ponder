package models

import "time"

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusBlocked    TaskStatus = "blocked"
)

type Task struct {
	ID                string     `json:"id"`
	FeatureID         string     `json:"feature_id"`
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	Specification     string     `json:"specification"`
	Priority          int        `json:"priority"`
	TestsRequired     bool       `json:"tests_required"`
	Status            TaskStatus `json:"status"`
	CompletionSummary *string    `json:"completion_summary"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	StartedAt         *time.Time `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at"`

	// FeatureName is a helper field for joined queries
	FeatureName string `json:"feature_name,omitempty"`
}
