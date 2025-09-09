package v1

import (
	"time"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/spec"
)

var (
	CtxKeyApplication = "applications"
)

// ApplicationBase application base model
type ApplicationBase struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Alias       string    `json:"alias"`
	Project     string    `json:"project"`
	Description string    `json:"description"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
	Icon        string    `json:"icon"`
	WorkflowId  string    `json:"workflow_id"`
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
	Name          string                      `json:"name" validate:"checkname"`
	Alias         string                      `json:"alias"`
	Version       string                      `json:"version"`
	Project       string                      `json:"project"`
	Description   string                      `json:"description" optional:"true"`
	Icon          string                      `json:"icon"`
	Component     []CreateComponentRequest    `json:"component"`
	WorkflowSteps []CreateWorkflowStepRequest `json:"workflow"`
}

type CreateComponentRequest struct {
	Name          string         `json:"name"`
	ComponentType config.JobType `json:"type"`
	Image         string         `json:"image,omitempty"` // Deprecated: use properties.image
	NameSpace     string         `json:"nameSpace"`
	Replicas      int32          `json:"replicas"`
	Properties    Properties     `json:"properties"`
	Traits        Traits         `json:"traits"`
}

type CreateWorkflowStepRequest struct {
	Name         string             `json:"name"`
	WorkflowType config.JobType     `json:"jobType"`
	Properties   WorkflowProperties `json:"properties"`
}

// ListApplicationResponse list applications by query params
type ListApplicationResponse struct {
	Applications []*ApplicationBase `json:"applications"`
}

type ApplicationsDeployRequest struct {
	WorkflowName string `json:"workflowName"`
	Name         string `json:"appName"`
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

type Properties = spec.Properties

type Traits = spec.Traits

type WorkflowProperties struct {
	Policies []string `json:"policies"`
}

type WorkflowTraits struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

type CreateWorkflowStepsRequest struct {
	Name               string           `json:"name"`
	ComponentType      config.JobType   `json:"jobType"`
	WorkflowProperties WorkflowPolicies `json:"properties"`
}

// ConfigMap相关API类型
type CreateConfigMapFromMapRequest struct {
	Name        string            `json:"name" validate:"required"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Data        map[string]string `json:"data" validate:"required"`
}

type CreateConfigMapFromURLRequest struct {
	Name        string            `json:"name" validate:"required"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	URL         string            `json:"url" validate:"required,url"`
	FileName    string            `json:"fileName,omitempty"`
}

type ConfigMapResponse struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Data        map[string]string `json:"data"`
	CreateTime  time.Time         `json:"createTime"`
	UpdateTime  time.Time         `json:"updateTime"`
}

type WorkflowPolicies struct {
	Policies []string `json:"policies"`
}

type CreateWorkflowResponse struct {
	WorkflowId string `json:"workflowId"`
}

type ExecWorkflowRequest struct {
	WorkflowId string `json:"workflowId" validate:"checkname"`
}

type ExecWorkflowResponse struct {
	WorkflowId string `json:"workflowId"`
}
