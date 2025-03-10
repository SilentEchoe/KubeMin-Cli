package v1

import "time"

// ApplicationBase application base model
type ApplicationBase struct {
	Name        string            `json:"name"`
	Alias       string            `json:"alias"`
	Project     *ProjectBase      `json:"project"`
	Description string            `json:"description"`
	CreateTime  time.Time         `json:"createTime"`
	UpdateTime  time.Time         `json:"updateTime"`
	Icon        string            `json:"icon"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	ReadOnly    bool              `json:"readOnly,omitempty"`
}

// ProjectBase project base model
type ProjectBase struct {
	Name        string    `json:"name"`
	Alias       string    `json:"alias"`
	Description string    `json:"description"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
	Owner       NameAlias `json:"owner,omitempty"`
	Namespace   string    `json:"namespace"`
}

// NameAlias name and alias
type NameAlias struct {
	Name  string `json:"name"`
	Alias string `json:"alias"`
}

// ListApplicationResponse list applications by query params
type ListApplicationResponse struct {
	Applications []*ApplicationBase `json:"applications"`
}

// ListApplicationOptions list application  query options
type ListApplicationOptions struct {
	Projects   []string          `json:"projects"`
	Env        string            `json:"env"`
	TargetName string            `json:"targetName"`
	Query      string            `json:"query"`
	Labels     map[string]string `json:"labels"`
}

// SimpleResponse simple response model for temporary
type SimpleResponse struct {
	Status string `json:"status"`
}

// WorkflowBase workflow base model
type WorkflowBase struct {
	Name        string         `json:"name"`
	Alias       string         `json:"alias"`
	Description string         `json:"description"`
	Enable      bool           `json:"enable"`
	Default     bool           `json:"default"`
	EnvName     string         `json:"envName"`
	CreateTime  time.Time      `json:"createTime"`
	UpdateTime  time.Time      `json:"updateTime"`
	Mode        string         `json:"mode"`
	SubMode     string         `json:"subMode"`
	Steps       []WorkflowStep `json:"steps,omitempty"`
}

// WorkflowStep workflow step config
type WorkflowStep struct {
	WorkflowStepBase `json:",inline"`
	Mode             string             `json:"mode,omitempty" validate:"checkMode"`
	SubSteps         []WorkflowStepBase `json:"subSteps,omitempty"`
}

// WorkflowStepBase is the step base of workflow
type WorkflowStepBase struct {
	Name string `json:"name" validate:"checkname"`
}
