package config

import (
	"strings"
	"time"
)

const (
	Redis           = "redis"
	TiDB            = "tidb"
	MySQL           = "mysql"
	DBNameKubeMinCLI = "kubemincli"
	SystemNamespace = "kubemin-system"
)

const (
	LabelCli           = "kube-min-cli"
	LabelAppID         = "kube-min-cli-appId"
	LabelComponentID   = "kube-min-cli-componentId"
	LabelComponentName = "kube-min-cli-componentName"
	LabelStorageRole   = "storage.kubemin.cli/pvc-role"
	LabelShareName     = "kubemin-share-name"
	LabelShareStrategy = "kubemin-share-strategy"
)

type JobRunPolicy string
type JobType string
type JobErrorPolicy string
type WorkflowTaskType string
type WorkflowMode string
type Status string
type ShareStrategy string

func (s Status) ToLower() Status {
	return Status(strings.ToLower(string(s)))
}

const (
	DefaultStorageMode = 420
	DefaultTaskRevoker = "system"
	DefaultNamespace   = "default"
	DeployTimeout      = 60 * 20 // 20 minutes
	DeleteTimeout  = 30 * time.Second
	JobNameRegex   = "^[a-z\u4e00-\u9fa5][a-z0-9\u4e00-\u9fa5-]{0,31}$"
	WorkflowRegex  = "^[a-zA-Z0-9-]+$"

	// ServerJob JobType 的类型分为几种：1.无状态服务 2.存储服务 3.网络服务
	ServerJob JobType = "webservice"
	StoreJob  JobType = "store"
	ConfJob   JobType = "config"
	SecretJob JobType = "secret"
	Service   JobType = "service"

	JobDeploy                   JobType = "deploy"
	JobDeployService            JobType = "service_deploy"
	JobDeployStore              JobType = "store_deploy"
	JobDeployPVC                JobType = "store_pvc_deploy"
	JobDeployConfigMap          JobType = "configmap_deploy"
	JobDeploySecret             JobType = "secret_deploy"
	JobDeployIngress            JobType = "ingress_deploy"
	JobDeployServiceAccount     JobType = "service_account_deploy"
	JobDeployRole               JobType = "role_deploy"
	JobDeployRoleBinding        JobType = "role_binding_deploy"
	JobDeployClusterRole        JobType = "cluster_role_deploy"
	JobDeployClusterRoleBinding JobType = "cluster_role_binding_deploy"

	DefaultRun    JobRunPolicy = ""
	DefaultNotRun JobRunPolicy = "default_not_run"
	ForceRun      JobRunPolicy = "force_run"
	SkipRun       JobRunPolicy = "skip"

	WorkflowTaskTypeWorkflow WorkflowTaskType = "workflow"
	WorkflowTaskTypeTesting  WorkflowTaskType = "test"
	WorkflowTaskTypeScanning WorkflowTaskType = "scan"
	WorkflowTaskTypeDelivery WorkflowTaskType = "delivery"

	WorkflowModeStepByStep WorkflowMode = "StepByStep"
	WorkflowModeDAG        WorkflowMode = "DAG"

	ShareStrategyDefault ShareStrategy = "default"
	ShareStrategyIgnore  ShareStrategy = "ignore"
	ShareStrategyForce   ShareStrategy = "force"

	WaitingTasksQueryTimeout      = 5 * time.Second
	TaskStateTransitionTimeout    = 5 * time.Second
	QueueDispatchTimeout          = 5 * time.Second
	DefaultLocalPollInterval      = 3 * time.Second
	DefaultDispatchPollInterval   = 3 * time.Second
	DefaultWorkerStaleInterval    = 15 * time.Second
	DefaultWorkerAutoClaimIdle    = 60 * time.Second
	DefaultWorkerAutoClaimCount   = 50
	DefaultWorkerReadCount        = 10
	DefaultWorkerReadBlock        = 2 * time.Second
	DefaultJobTaskTimeout         = 20 * time.Minute //超时时间设置为20分钟
	DefaultMaxConcurrentWorkflows = 10

	// Worker resilience settings
	DefaultWorkerBackoffMin       = 200 * time.Millisecond // 最小退避时间
	DefaultWorkerBackoffMax       = 5 * time.Minute        // 最大退避时间
	DefaultWorkerMaxReadFailures  = 10                     // 连续 10 次失败后退出
	DefaultWorkerMaxClaimFailures = 10                     // 连续 10 次失败后退出
)

const (
	StatusCompleted      Status = "completed"                      //执行完毕
	StatusDisabled       Status = "disabled"                       //已关闭
	StatusCreated        Status = "created"                        //创建
	StatusRunning        Status = "running"                        //运行中
	StatusPassed         Status = "passed"                         //通过
	StatusSkipped        Status = "skipped"                        //跳过
	StatusFailed         Status = "failed"                         //错误
	StatusTimeout        Status = "timeout"                        //超时
	StatusCancelled      Status = "cancelled"                      //取消
	StatusPause          Status = "pause"                          //暂停
	StatusWaiting        Status = "waiting"                        //等待中
	StatusQueued         Status = "queued"                         //排队中
	StatusBlocked        Status = "blocked"                        //阻塞
	QueueItemPending     Status = "pending"                        //等待调度
	StatusChanged        Status = "changed"                        //改变
	StatusNotRun         Status = "not_run"                        //没有运行
	StatusPrepare        Status = "prepare"                        //准备
	StatusReject         Status = "reject"                         //拒绝
	StatusDistributed    Status = "distributed"                    //分布式
	StatusWaitingApprove Status = "wait_for_approval"              //等待批准
	StatusDebugBefore    Status = "debug_before"                   //调试开始
	StatusDebugAfter     Status = "debug_after"                    //调试之后
	StatusUnstable       Status = "unstable"                       //不稳定
	StatusManualApproval Status = "wait_for_manual_error_handling" //等待手动错误处理
)

// ComponentStatus 组件运行时状态（由 Informer 同步）
type ComponentStatus string

const (
	ComponentStatusRunning ComponentStatus = "Running" // 运行中（所有副本就绪）
	ComponentStatusPending ComponentStatus = "Pending" // 部分副本就绪或正在启动
	ComponentStatusFailed  ComponentStatus = "Failed"  // 失败
	ComponentStatusUnknown ComponentStatus = "Unknown" // 未知状态
)

const (
	JobPriorityMaxHigh = 0
	// JobPriorityHigh defines the high priority level, for resources like PVC, ConfigMap, Secret.
	JobPriorityHigh = 1
	// JobPriorityNormal defines the normal priority level, for resources like Deployments, StatefulSets.
	JobPriorityNormal = 10
	// JobPriorityLow defines the low priority level, for cleanup or notification jobs.
	JobPriorityLow = 20
)

// ParseWorkflowMode normalizes workflow mode values, defaulting to StepByStep when empty or unknown.
func ParseWorkflowMode(mode string) WorkflowMode {
	switch WorkflowMode(mode) {
	case WorkflowModeDAG:
		return WorkflowModeDAG
	case WorkflowModeStepByStep:
		return WorkflowModeStepByStep
	default:
		return WorkflowModeStepByStep
	}
}

// IsParallel reports whether the workflow mode permits parallel execution.
func (m WorkflowMode) IsParallel() bool {
	return m == WorkflowModeDAG
}

// 用户侧声明的存储类型（API 入参）
const (
	StorageTypePersistent  = "persistent"
	StorageTypeEphemeral   = "ephemeral"
	StorageTypeHostMounted = "host-mounted"
	StorageTypeConfig      = "config"
	StorageTypeSecret      = "secret"
)

// Kubernetes 中内部映射的 Volume 类型
const (
	VolumeTypePVC       = "pvc"
	VolumeTypeEmptyDir  = "emptyDir"
	VolumeTypeHostPath  = "hostPath"
	VolumeTypeConfigMap = "configMap"
	VolumeTypeSecret    = "secret"
)

var StorageTypeMapping = map[string]string{
	StorageTypePersistent:  VolumeTypePVC,
	StorageTypeEphemeral:   VolumeTypeEmptyDir,
	StorageTypeHostMounted: VolumeTypeHostPath,
	StorageTypeConfig:      VolumeTypeConfigMap,
	StorageTypeSecret:      VolumeTypeSecret,
}

// UpdateStrategy 版本更新策略类型
type UpdateStrategy string

const (
	// UpdateStrategyRolling 滚动更新（默认）- 逐步替换Pod，保证服务可用性
	UpdateStrategyRolling UpdateStrategy = "rolling"
	// UpdateStrategyRecreate 重建更新 - 先删除所有旧Pod，再创建新Pod
	UpdateStrategyRecreate UpdateStrategy = "recreate"
	// UpdateStrategyCanary 金丝雀更新 - 先更新部分Pod，验证后再全量更新
	UpdateStrategyCanary UpdateStrategy = "canary"
	// UpdateStrategyBlueGreen 蓝绿部署 - 创建新版本，切换流量后销毁旧版本
	UpdateStrategyBlueGreen UpdateStrategy = "blue-green"
)

// ParseUpdateStrategy 解析更新策略，默认返回滚动更新
func ParseUpdateStrategy(strategy string) UpdateStrategy {
	switch UpdateStrategy(strategy) {
	case UpdateStrategyRecreate:
		return UpdateStrategyRecreate
	case UpdateStrategyCanary:
		return UpdateStrategyCanary
	case UpdateStrategyBlueGreen:
		return UpdateStrategyBlueGreen
	case UpdateStrategyRolling:
		return UpdateStrategyRolling
	default:
		return UpdateStrategyRolling
	}
}

// ComponentAction 组件操作类型
type ComponentAction string

const (
	// ComponentActionUpdate 更新组件（默认）
	ComponentActionUpdate ComponentAction = "update"

	// ComponentActionAdd 新增组件
	ComponentActionAdd ComponentAction = "add"

	// ComponentActionRemove 删除组件
	ComponentActionRemove ComponentAction = "remove"
)

// ParseComponentAction 解析组件操作类型，默认返回更新
func ParseComponentAction(action string) ComponentAction {
	switch ComponentAction(action) {
	case ComponentActionAdd:
		return ComponentActionAdd
	case ComponentActionRemove:
		return ComponentActionRemove
	case ComponentActionUpdate:
		return ComponentActionUpdate
	default:
		return ComponentActionUpdate
	}
}
