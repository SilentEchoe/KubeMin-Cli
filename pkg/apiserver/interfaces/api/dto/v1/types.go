package v1

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"time"
)

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

type CreateWorkflowRequest struct {
	Name        string                       `json:"name" validate:"checkname"`
	Alias       string                       `json:"alias"`
	Project     string                       `json:"project" validate:"checkname"`
	Description string                       `json:"description" optional:"true"`
	Labels      map[string]string            `json:"labels,omitempty"`
	Component   []CreateComponentRequest     `json:"component"`
	Workflows   []CreateWorkflowStepsRequest `json:"workflow"`
}

type CreateComponentRequest struct {
	Name          string         `json:"name"`
	ComponentType config.JobType `json:"type"`
	Properties    Properties     `json:"properties"`
}

type Properties struct {
	Image string  `json:"image"`
	Ports []Ports `json:"ports"`
}

type Ports struct {
	Port   int64 `json:"port"`
	Expose bool  `json:"expose"`
}

type CreateWorkflowStepsRequest struct {
	Name               string           `json:"name"`
	ComponentType      config.JobType   `json:"jobType"`
	WorkflowProperties WorkflowPolicies `json:"properties"`
}

type WorkflowPolicies struct {
	Policies []string `json:"policies"`
}

type CreateWorkflowResponse struct {
	WorkflowId string `json:"workflowId"`
}
