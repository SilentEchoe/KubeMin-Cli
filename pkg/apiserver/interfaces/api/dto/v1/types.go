package v1

import "time"

var (
	CtxKeyApplication = "applications"
)

// ApplicationBase application base model
type ApplicationBase struct {
	Name        string    `json:"name"`
	Alias       string    `json:"alias"`
	Description string    `json:"description"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
	Icon        string    `json:"icon"`
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

type CreateApplicationsRequest struct {
	Name        string            `json:"name"`
	Alias       string            `json:"alias"`
	Project     string            `json:"project"`
	Description string            `json:"description"`
	Icon        string            `json:"icon"`
	Labels      map[string]string `json:"labels,omitempty"`
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

type ApplicationsDeployRequest struct {
	WorkflowName string `json:"workflowName"`
	Name         string `json:"appName"`
}

type ApplicationsDeployResponse struct {
	CreateTime time.Time `json:"createTime"`
	Version    string    `json:"version"`
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
