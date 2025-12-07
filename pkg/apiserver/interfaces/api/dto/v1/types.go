package v1

import (
	"time"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/spec"
)

// ApplicationBase application base model
type ApplicationBase struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Alias       string    `json:"alias"`
	Project     string    `json:"project"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
	Icon        string    `json:"icon"`
	WorkflowID  string    `json:"workflow_id"`
	TmpEnable   bool      `json:"tmp_enable"`
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
	ID            string                      `json:"ID"`
	Name          string                      `json:"name" validate:"checkname"`
	NameSpace     string                      `json:"namespace"`
	Image         string                      `json:"image"`
	Alias         string                      `json:"alias"`
	Version       string                      `json:"version"`
	Project       string                      `json:"project"`
	Description   string                      `json:"description" optional:"true"`
	Icon          string                      `json:"icon"`
	Component     []CreateComponentRequest    `json:"component"`
	WorkflowSteps []CreateWorkflowStepRequest `json:"workflow"`

	// TmpEnable 标记该应用是否允许作为模板被引用
	TmpEnable *bool `json:"tmp_enable,omitempty"`
}

type CreateComponentRequest struct {
	Name          string         `json:"name"`
	ComponentType config.JobType `json:"type"`
	Image         string         `json:"image,omitempty"`
	NameSpace     string         `json:"nameSpace"`
	Replicas      int32          `json:"replicas"`
	Properties    Properties     `json:"properties"`
	Traits        Traits         `json:"traits"`
	Template      *TemplateRef   `json:"Tem,omitempty"`
}

type TemplateRef struct {
	ID     string `json:"id"`
	Target string `json:"target,omitempty"` // 目标模板组件名，用于精确匹配
}

type CreateWorkflowStepRequest struct {
	Name         string                         `json:"name"`
	WorkflowType config.JobType                 `json:"jobType,omitempty"`
	Properties   WorkflowProperties             `json:"properties,omitempty"`
	Components   []string                       `json:"components,omitempty"`
	Mode         string                         `json:"mode,omitempty"`
	SubSteps     []CreateWorkflowSubStepRequest `json:"subSteps,omitempty"`
}

// ListApplicationResponse list applications by query params
type ListApplicationResponse struct {
	Applications []*ApplicationBase `json:"applications"`
}

type ApplicationsDeployRequest struct {
	WorkflowName string `json:"workflowName"`
	Name         string `json:"appName"`
}

type CleanupApplicationResourcesResponse struct {
	AppID            string   `json:"appId"`
	DeletedResources []string `json:"deletedResources"`
	FailedResources  []string `json:"failedResources,omitempty"`
}

type CreateWorkflowRequest struct {
	Name        string                       `json:"name" validate:"checkname"`
	Project     string                       `json:"project" validate:"checkname"`
	Alias       string                       `json:"alias"`
	Description string                       `json:"description" optional:"true"`
	Labels      map[string]string            `json:"labels,omitempty"`
	Component   []CreateComponentRequest     `json:"component"`
	Workflows   []CreateWorkflowStepsRequest `json:"workflow"`
	TryRun      bool                         `json:"tryRun"`
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
	Name               string                         `json:"name"`
	ComponentType      config.JobType                 `json:"jobType,omitempty"`
	WorkflowProperties WorkflowPolicies               `json:"properties,omitempty"`
	Components         []string                       `json:"components,omitempty"`
	Mode               string                         `json:"mode,omitempty"`
	SubSteps           []CreateWorkflowSubStepRequest `json:"subSteps,omitempty"`
}

type CreateWorkflowSubStepRequest struct {
	Name         string             `json:"name"`
	WorkflowType config.JobType     `json:"jobType,omitempty"`
	Properties   WorkflowProperties `json:"properties,omitempty"`
	Components   []string           `json:"components,omitempty"`
}

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
	WorkflowID string `json:"workflowId"`
}

type UpdateApplicationWorkflowRequest struct {
	WorkflowID string                      `json:"workflowId,omitempty"`
	Name       string                      `json:"name,omitempty"`
	Alias      string                      `json:"alias,omitempty"`
	Workflow   []CreateWorkflowStepRequest `json:"workflow" validate:"required,min=1,dive"`
}

type UpdateWorkflowResponse struct {
	WorkflowID string `json:"workflowId"`
}

type ExecWorkflowRequest struct {
	WorkflowID string `json:"workflowId" validate:"checkname"`
}

type ExecWorkflowResponse struct {
	TaskID string `json:"taskId"`
}

type CancelWorkflowRequest struct {
	TaskID string `json:"taskId" validate:"required"`
	User   string `json:"user,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type CancelWorkflowResponse struct {
	TaskID string `json:"taskId"`
	Status string `json:"status"`
}

type TaskStatusResponse struct {
	TaskID       string                  `json:"taskId"`
	Status       string                  `json:"status"`
	WorkflowID   string                  `json:"workflowId,omitempty"`
	WorkflowName string                  `json:"workflowName,omitempty"`
	AppID        string                  `json:"appId,omitempty"`
	Type         config.WorkflowTaskType `json:"type,omitempty"`
	Components   []ComponentTaskStatus   `json:"components,omitempty"`
}

type ComponentTaskStatus struct {
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	StartTime int64  `json:"startTime,omitempty"`
	EndTime   int64  `json:"endTime,omitempty"`
}

type ListApplicationWorkflowsResponse struct {
	Workflows []*ApplicationWorkflow `json:"workflows"`
}

type ListApplicationComponentsResponse struct {
	Components []*ApplicationComponent `json:"components"`
}

type ApplicationWorkflow struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Alias        string                  `json:"alias"`
	Namespace    string                  `json:"namespace,omitempty"`
	ProjectID    string                  `json:"projectId,omitempty"`
	Description  string                  `json:"description,omitempty"`
	Status       string                  `json:"status"`
	Disabled     bool                    `json:"disabled"`
	Steps        []WorkflowStepDetail    `json:"steps,omitempty"`
	CreateTime   time.Time               `json:"createTime"`
	UpdateTime   time.Time               `json:"updateTime"`
	WorkflowType config.WorkflowTaskType `json:"workflowType"`
}

type WorkflowStepDetail struct {
	Name         string                  `json:"name"`
	WorkflowType config.JobType          `json:"workflowType,omitempty"`
	Mode         config.WorkflowMode     `json:"mode,omitempty"`
	Components   []string                `json:"components,omitempty"`
	SubSteps     []WorkflowSubStepDetail `json:"subSteps,omitempty"`
}

type WorkflowSubStepDetail struct {
	Name         string         `json:"name"`
	WorkflowType config.JobType `json:"workflowType,omitempty"`
	Components   []string       `json:"components,omitempty"`
}

type ApplicationComponent struct {
	ID            int            `json:"id"`
	AppID         string         `json:"appId"`
	Name          string         `json:"name"`
	Namespace     string         `json:"namespace"`
	Image         string         `json:"image,omitempty"`
	Replicas      int32          `json:"replicas"`
	ComponentType config.JobType `json:"type"`
	Properties    Properties     `json:"properties"`
	Traits        Traits         `json:"traits"`
	CreateTime    time.Time      `json:"createTime"`
	UpdateTime    time.Time      `json:"updateTime"`
}

// UpdateVersionRequest 版本更新请求
type UpdateVersionRequest struct {
	// Version 新版本号（必填）
	Version string `json:"version" validate:"required"`

	// Strategy 更新策略：rolling（默认）、recreate、canary、blue-green
	Strategy string `json:"strategy,omitempty"`

	// Components 要更新的组件列表（可选，不填则仅更新版本号）
	Components []ComponentUpdateSpec `json:"components,omitempty"`

	// AutoExec 是否自动执行工作流，默认 true
	AutoExec *bool `json:"autoExec,omitempty"`

	// Description 更新说明
	Description string `json:"description,omitempty"`
}

// ComponentUpdateSpec 组件更新规格
type ComponentUpdateSpec struct {
	// Action 操作类型：update（默认）、add、remove
	Action string `json:"action,omitempty"`

	// Name 组件名称
	Name string `json:"name" validate:"required"`

	// 以下字段仅在 action 为 update 或 add 时有效

	// Image 新镜像地址（可选）
	Image string `json:"image,omitempty"`

	// Replicas 新副本数（可选）
	Replicas *int32 `json:"replicas,omitempty"`

	// Env 环境变量覆盖（可选，合并更新）
	Env map[string]string `json:"env,omitempty"`

	// 以下字段仅在 action 为 add 时需要

	// ComponentType 组件类型（新增时必填）
	ComponentType config.JobType `json:"type,omitempty"`

	// Properties 组件属性（新增时可选）
	Properties *Properties `json:"properties,omitempty"`

	// Traits 组件特性（新增时可选）
	Traits *Traits `json:"traits,omitempty"`
}

// UpdateVersionResponse 版本更新响应
type UpdateVersionResponse struct {
	// AppID 应用ID
	AppID string `json:"appId"`

	// Version 新版本号
	Version string `json:"version"`

	// PreviousVersion 更新前版本号
	PreviousVersion string `json:"previousVersion"`

	// Strategy 使用的更新策略
	Strategy string `json:"strategy"`

	// TaskID 工作流任务ID（如果触发了工作流执行）
	TaskID string `json:"taskId,omitempty"`

	// UpdatedComponents 已更新的组件名称列表
	UpdatedComponents []string `json:"updatedComponents,omitempty"`

	// AddedComponents 新增的组件名称列表
	AddedComponents []string `json:"addedComponents,omitempty"`

	// RemovedComponents 已删除的组件名称列表
	RemovedComponents []string `json:"removedComponents,omitempty"`
}
