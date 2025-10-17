package model

import "KubeMin-Cli/pkg/apiserver/config"

func init() {
	RegisterModel(&Workflow{})
}

// Workflow application delivery database model
type Workflow struct {
	ID           string                  `json:"id" gorm:"primaryKey"`
	Name         string                  `json:"name" `
	Namespace    string                  `json:"namespace"`
	Alias        string                  `json:"alias"`    //别名
	Disabled     bool                    `json:"disabled"` //是否关闭，创建时默认为false
	ProjectID    string                  `json:"project_id"`
	AppID        string                  `gorm:"column:appid" json:"app_id"`
	UserID       string                  `json:"user_id"`
	Description  string                  `json:"description"`
	WorkflowType config.WorkflowTaskType `gorm:"column:workflow_type" json:"workflow_type"` //工作流类型
	Status       config.Status           `json:"status"`                                    //分为开启和关闭等状态
	Steps        *JSONStruct             `json:"steps,omitempty" gorm:"serializer:json"`
	BaseModel
}

type WorkflowSteps struct {
	Steps []*WorkflowStep `json:"steps"`
}

type WorkflowStep struct {
	Name         string              `json:"name"`
	Level        int                 `json:"level,omitempty"`
	WorkflowType config.JobType      `json:"workflowType,omitempty"`
	Mode         config.WorkflowMode `json:"mode,omitempty"`
	Properties   []Policies          `json:"properties,omitempty"`
	SubSteps     []*WorkflowSubStep  `json:"subSteps,omitempty"`
}

type WorkflowSubStep struct {
	Name         string         `json:"name"`
	WorkflowType config.JobType `json:"workflowType,omitempty"`
	Properties   []Policies     `json:"properties,omitempty"`
}

// ComponentNames returns referenced component names for a workflow step.
func (w *WorkflowStep) ComponentNames() []string {
	if w == nil {
		return nil
	}
	names := extractPolicyNames(w.Properties)
	if len(names) == 0 && w.Name != "" {
		names = append(names, w.Name)
	}
	return names
}

// ComponentNames returns referenced component names for a workflow sub-step.
func (w *WorkflowSubStep) ComponentNames() []string {
	if w == nil {
		return nil
	}
	names := extractPolicyNames(w.Properties)
	if len(names) == 0 && w.Name != "" {
		names = append(names, w.Name)
	}
	return names
}

func extractPolicyNames(policies []Policies) []string {
	if len(policies) == 0 {
		return nil
	}
	var names []string
	for _, policy := range policies {
		names = append(names, policy.Policies...)
	}
	return names
}

type Policies struct {
	Policies []string `json:"policies"`
}

func (w *Workflow) PrimaryKey() string {
	return w.ID
}

func (w *Workflow) TableName() string {
	return tableNamePrefix + "workflow"
}

func (w *Workflow) ShortTableName() string {
	return "workflow"
}

func (w *Workflow) Index() map[string]interface{} {
	index := make(map[string]interface{})
	if w.ID != "" {
		index["id"] = w.ID
	}
	if w.Name != "" {
		index["name"] = w.Name
	}
	return index
}
