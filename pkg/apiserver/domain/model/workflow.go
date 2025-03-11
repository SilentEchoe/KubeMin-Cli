package model

// Workflow application delivery database model
type Workflow struct {
	BaseModel
	Name        string `json:"name" gorm:"primaryKey"`
	Alias       string `json:"alias"`
	Description string `json:"description"`
	// Workflow used by the default
	Default       *bool          `json:"default"`
	AppPrimaryKey string         `json:"appPrimaryKey" gorm:"primaryKey"`
	EnvName       string         `json:"envName"`
	Steps         []WorkflowStep `json:"steps,omitempty" gorm:"serializer:json"`
}

// WorkflowStep defines how to execute a workflow step.
type WorkflowStep struct {
	WorkflowStepBase `json:",inline" bson:",inline"`
	SubSteps         []WorkflowStepBase `json:"subSteps,omitempty"`
}

// WorkflowStepBase is the step base of workflow
type WorkflowStepBase struct {
	// Name是工作流步骤的唯一名称。
	Name    string `json:"name"`
	Alias   string `json:"alias"`
	Type    string `json:"type"`
	Timeout string `json:"timeout,omitempty"`
}
