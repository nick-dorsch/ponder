package models

type Dependency struct {
	TaskID          string `json:"task_id"`
	DependsOnTaskID string `json:"depends_on_task_id"`

	// Helper fields for staging/resolution
	TaskName             string `json:"task_name,omitempty"`
	FeatureName          string `json:"feature_name,omitempty"`
	DependsOnTaskName    string `json:"depends_on_task_name,omitempty"`
	DependsOnFeatureName string `json:"depends_on_feature_name,omitempty"`
}
