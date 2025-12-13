package model

import (
	"time"
)

// Template 应用模板定义
type Template struct {
	ID          string `gorm:"primaryKey;type:varchar(24)" json:"id"`
	Name        string `gorm:"type:varchar(255);uniqueIndex;not null" json:"name"`
	DisplayName string `gorm:"type:varchar(64)" json:"display_name"`
	Description string `gorm:"type:text" json:"description"`
	Category    string `gorm:"type:varchar(64);index" json:"category"`
	Version     string `gorm:"type:varchar(64);default:'1.0.0'" json:"version"`
	Icon        string `gorm:"type:varchar(255)" json:"icon"`
	Author      string `gorm:"type:varchar(128)" json:"author"`
	Source      string `gorm:"type:varchar(255)" json:"source"`
	Tags        string `gorm:"type:varchar(512)" json:"tags"`

	// 模板内容
	Parameters TemplateParameters `gorm:"type:json" json:"parameters"`
	Components TemplateComponents `gorm:"type:json" json:"components"`
	Workflow   TemplateWorkflow   `gorm:"type:json" json:"workflow"`

	// 元数据
	IsPublic bool   `gorm:"default:false;index" json:"is_public"`
	IsSystem bool   `gorm:"default:false;index" json:"is_system"`
	Status   string `gorm:"type:varchar(32);default:'active';index" json:"status"`

	// 时间戳
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TemplateParameters 模板参数定义
type TemplateParameters struct {
	// 参数定义列表
	Definitions []ParameterDefinition `json:"definitions"`

	// 参数分组
	Groups []ParameterGroup `json:"groups"`

	// 参数验证规则
	Validation *ParameterValidation `json:"validation,omitempty"`
}

// ParameterDefinition 单个参数定义
type ParameterDefinition struct {
	Name        string            `json:"name"`                  // 参数名称 (变量名)
	DisplayName string            `json:"display_name"`          // 显示名称
	Description string            `json:"description,omitempty"` // 参数描述
	Type        string            `json:"type"`                  // 参数类型: string, int, bool, array, object
	Default     interface{}       `json:"default,omitempty"`     // 默认值
	Required    bool              `json:"required"`              // 是否必需
	Validation  *ParameterRule    `json:"validation,omitempty"`  // 验证规则
	Options     []ParameterOption `json:"options,omitempty"`     // 可选值（用于下拉选择）
	DependsOn   []string          `json:"depends_on,omitempty"`  // 依赖的其他参数
	Group       string            `json:"group,omitempty"`       // 所属分组
	Order       int               `json:"order,omitempty"`       // 显示顺序
	Advanced    bool              `json:"advanced,omitempty"`    // 是否为高级参数
}

// ParameterGroup 参数分组
type ParameterGroup struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description,omitempty"`
	Order       int    `json:"order,omitempty"`
}

// ParameterRule 参数验证规则
type ParameterRule struct {
	MinLength     *int     `json:"min_length,omitempty"`     // 最小长度
	MaxLength     *int     `json:"max_length,omitempty"`     // 最大长度
	MinValue      *int     `json:"min_value,omitempty"`      // 最小值
	MaxValue      *int     `json:"max_value,omitempty"`      // 最大值
	Pattern       string   `json:"pattern,omitempty"`        // 正则表达式
	Enum          []string `json:"enum,omitempty"`           // 枚举值
	CustomMessage string   `json:"custom_message,omitempty"` // 自定义错误消息
}

// ParameterOption 参数选项
type ParameterOption struct {
	Label string      `json:"label"`
	Value interface{} `json:"value"`
}

// ParameterValidation 参数全局验证
type ParameterValidation struct {
	RequiredParams []string     `json:"required_params,omitempty"`
	CustomRules    []CustomRule `json:"custom_rules,omitempty"`
}

// CustomRule 自定义验证规则
type CustomRule struct {
	Name       string `json:"name"`
	Expression string `json:"expression"` // JavaScript表达式
	Message    string `json:"message"`
}

// TemplateComponents 模板组件定义
type TemplateComponents struct {
	// 组件模板列表
	Components []ComponentTemplate `json:"components"`

	// 组件关系定义
	Relations []ComponentRelation `json:"relations,omitempty"`

	// 命名规则
	NamingRules *NamingRules `json:"naming_rules,omitempty"`
}

// ComponentTemplate 组件模板
type ComponentTemplate struct {
	Name          string                 `json:"name"`                    // 模板名称 (可包含变量)
	Type          string                 `json:"type"`                    // 组件类型
	Description   string                 `json:"description,omitempty"`   // 组件描述
	Properties    map[string]interface{} `json:"properties"`              // 属性模板 (可包含变量)
	Traits        map[string]interface{} `json:"traits,omitempty"`        // Trait模板 (可包含变量)
	Conditions    []string               `json:"conditions,omitempty"`    // 显示条件
	Validation    *ComponentValidation   `json:"validation,omitempty"`    // 组件验证
	Documentation *ComponentDocs         `json:"documentation,omitempty"` // 组件文档
}

// ComponentRelation 组件关系
type ComponentRelation struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Type     string `json:"type"` // depends_on, connects_to, exposes_to
	Optional bool   `json:"optional,omitempty"`
}

// NamingRules 命名规则
type NamingRules struct {
	Prefix    string `json:"prefix,omitempty"`
	Suffix    string `json:"suffix,omitempty"`
	Separator string `json:"separator,omitempty"` // 默认: "-"
	MaxLength int    `json:"max_length,omitempty"`
	Transform string `json:"transform,omitempty"` // lowercase, uppercase, camelCase
}

// ComponentValidation 组件验证
type ComponentValidation struct {
	RequiredTraits  []string `json:"required_traits,omitempty"`
	ForbiddenTraits []string `json:"forbidden_traits,omitempty"`
	MinReplicas     *int     `json:"min_replicas,omitempty"`
	MaxReplicas     *int     `json:"max_replicas,omitempty"`
}

// ComponentDocs 组件文档
type ComponentDocs struct {
	Brief       string   `json:"brief,omitempty"`
	Description string   `json:"description,omitempty"`
	Usage       string   `json:"usage,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

// TemplateWorkflow 模板工作流定义
type TemplateWorkflow struct {
	// 工作流步骤模板
	Steps []WorkflowStepTemplate `json:"steps"`

	// 执行策略
	Strategy *WorkflowStrategy `json:"strategy,omitempty"`

	// 参数化配置
	Parameterization *WorkflowParameterization `json:"parameterization,omitempty"`
}

// WorkflowStepTemplate 工作流步骤模板
type WorkflowStepTemplate struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"` // deploy, configure, validate, wait
	Components []string               `json:"components,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Conditions []string               `json:"conditions,omitempty"`
	Timeout    *int                   `json:"timeout,omitempty"`
	Retry      *RetryPolicy           `json:"retry,omitempty"`
}

// WorkflowStrategy 工作流策略
type WorkflowStrategy struct {
	Mode         string `json:"mode"` // StepByStep, DAG
	Parallelism  *int   `json:"parallelism,omitempty"`
	FailFast     bool   `json:"fail_fast,omitempty"`
	RollbackMode string `json:"rollback_mode,omitempty"` // none, automatic, manual
}

// WorkflowParameterization 工作流参数化
type WorkflowParameterization struct {
	// 步骤级别的参数映射
	StepMappings map[string]map[string]string `json:"step_mappings,omitempty"`

	// 条件参数化
	ConditionalSteps []ConditionalStep `json:"conditional_steps,omitempty"`
}

// ConditionalStep 条件步骤
type ConditionalStep struct {
	StepName   string                 `json:"step_name"`
	Condition  string                 `json:"condition"` // JavaScript表达式
	Parameters map[string]interface{} `json:"parameters"`
}

// RetryPolicy 重试策略
type RetryPolicy struct {
	Attempts      int     `json:"attempts"`
	DelaySeconds  int     `json:"delay_seconds"`
	BackoffFactor float64 `json:"backoff_factor,omitempty"`
	MaxDelay      int     `json:"max_delay,omitempty"`
}

// TemplateVersion 模板版本管理
type TemplateVersion struct {
	ID          string          `gorm:"primaryKey;type:varchar(24)" json:"id"`
	TemplateID  string          `gorm:"type:varchar(24);index;not null" json:"template_id"`
	Version     string          `gorm:"type:varchar(64);index;not null" json:"version"`
	Description string          `gorm:"type:text" json:"description"`
	Content     TemplateContent `gorm:"type:json" json:"content"`
	IsCurrent   bool            `gorm:"default:false;index" json:"is_current"`
	CreatedAt   time.Time       `json:"created_at"`
	CreatedBy   string          `gorm:"type:varchar(128)" json:"created_by"`
}

// TemplateContent 模板内容快照
type TemplateContent struct {
	Parameters TemplateParameters `json:"parameters"`
	Components TemplateComponents `json:"components"`
	Workflow   TemplateWorkflow   `json:"workflow"`
}

// TemplateInstance 模板实例（基于模板创建的应用）
type TemplateInstance struct {
	ID         string                 `gorm:"primaryKey;type:varchar(24)" json:"id"`
	TemplateID string                 `gorm:"type:varchar(24);index;not null" json:"template_id"`
	Version    string                 `gorm:"type:varchar(64)" json:"version"`
	AppID      string                 `gorm:"type:varchar(24);index;not null" json:"app_id"`
	Parameters map[string]interface{} `gorm:"type:json" json:"parameters"`
	CreatedAt  time.Time              `json:"created_at"`
}

// TemplateCategory 模板分类
type TemplateCategory struct {
	ID          string    `gorm:"primaryKey;type:varchar(24)" json:"id"`
	Name        string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"name"`
	DisplayName string    `gorm:"type:varchar(128)" json:"display_name"`
	Description string    `gorm:"type:text" json:"description"`
	Icon        string    `gorm:"type:varchar(255)" json:"icon"`
	ParentID    string    `gorm:"type:varchar(24);index" json:"parent_id,omitempty"`
	SortOrder   int       `gorm:"default:0" json:"sort_order"`
	IsSystem    bool      `gorm:"default:false" json:"is_system"`
	Status      string    `gorm:"type:varchar(32);default:'active'" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// 常量定义
const (
	// 参数类型
	ParameterTypeString = "string"
	ParameterTypeInt    = "int"
	ParameterTypeBool   = "bool"
	ParameterTypeArray  = "array"
	ParameterTypeObject = "object"

	// 模板状态
	TemplateStatusActive   = "active"
	TemplateStatusDraft    = "draft"
	TemplateStatusArchived = "archived"

	// 组件关系类型
	RelationTypeDependsOn  = "depends_on"
	RelationTypeConnectsTo = "connects_to"
	RelationTypeExposesTo  = "exposes_to"

	// 命名转换
	NamingTransformLowercase = "lowercase"
	NamingTransformUppercase = "uppercase"
	NamingTransformCamelCase = "camelCase"
	NamingTransformKebabCase = "kebab-case"
	NamingTransformSnakeCase = "snake_case"
)
