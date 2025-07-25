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
	Project       string                      `json:"project" validate:"checkname"`
	Description   string                      `json:"description" optional:"true"`
	Icon          string                      `json:"icon"`
	Component     []CreateComponentRequest    `json:"component"`
	WorkflowSteps []CreateWorkflowStepRequest `json:"workflow"`
}

type CreateComponentRequest struct {
	Name          string         `json:"name"`
	ComponentType config.JobType `json:"type"`
	Image         string         `json:"image"`
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

type Properties struct {
	Image  string            `json:"image"`
	Ports  []Ports           `json:"ports"`
	Env    map[string]string `json:"env"`
	Labels map[string]string `json:"labels"`
}

type Traits struct {
	Storage []StorageSpec   `json:"storage"`           //存储特性
	Config  []ConfigMapSpec `json:"config"`            //配置文件
	Secret  []SecretSpec    `json:"secret"`            //密钥信息
	Sidecar []SidecarSpec   `json:"sidecar,omitempty"` //容器边车
}

type StorageSpec struct {
	Type      string `json:"type"`
	MountPath string `json:"mountPath"`
	Size      string `json:"size"`
	Name      string `json:"name"`
}

type ConfigMapSpec struct {
	Data map[string]string `json:"data"`
}

type SecretSpec struct {
	Data map[string]string `json:"data"`
}

type SidecarSpec struct {
	Name    string            `json:"name"`  //如果用户不填写，可以根据组件的名称来自定义
	Image   string            `json:"image"` //必填镜像
	Command []string          `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Traits  Traits            `json:"mounts,omitempty"` //可以附加各种特征，但是边车容器内不能附加边车容器，这点需要进行校验
}

type Ports struct {
	Port   int64 `json:"port"`
	Expose bool  `json:"expose"`
}

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
