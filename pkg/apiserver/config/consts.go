package config

import (
	"strings"
	"time"
)

const (
	REDIS             = "redis"
	TIDB              = "tidb"
	MYSQL             = "mysql"
	DBNAME_KUBEMINCLI = "kubemincli"
	NAMESPACE         = "min-cli-system"
)

const (
	LabelCli         = "kube-min-cli"
	LabelAppID       = "kube-min-cli-appId"
	LabelComponentID = "kube-min-cli-componentId"
	LabelStorageRole = "storage.kubemin.cli/pvc-role"
)

type JobRunPolicy string
type JobType string
type JobErrorPolicy string
type WorkflowTaskType string
type WorkflowMode string
type Status string

func (s Status) ToLower() Status {
	return Status(strings.ToLower(string(s)))
}

const (
	DefaultStorageMode = 420
	DefaultTaskRevoker = "system"
	DefaultNamespace   = "default"
	DeployTimeout      = 60 * 20 // 20 minutes
	DelTimeOut         = 30 * time.Second
	JobNameRegx        = "^[a-z\u4e00-\u9fa5][a-z0-9\u4e00-\u9fa5-]{0,31}$"
	WorkflowRegx       = "^[a-zA-Z0-9-]+$"

	// ServerJob JobType 的类型分为几种：1.无状态服务 2.存储服务 3.网络服务
	ServerJob JobType = "webservice"
	StoreJob  JobType = "store"
	Service   JobType = "service"
	ConfJob   JobType = "config"
	SecretJob JobType = "secret"

	JobDeploy                   JobType = "deploy"
	JobDeployService            JobType = "service_deploy"
	JobDeployStore              JobType = "store_deploy"
	JobDeployPVC                JobType = "store_pvc_deploy"
	JobDeployConfigMap          JobType = "configmap_deploy"
	JobDeploySecret             JobType = "secret_deploy"
	JobDeployIngress            JobType = "ingress_deploy"
	JobDeployServiceAccount     JobType = "serviceaccount_deploy"
	JobDeployRole               JobType = "role_deploy"
	JobDeployRoleBinding        JobType = "rolebinding_deploy"
	JobDeployClusterRole        JobType = "clusterrole_deploy"
	JobDeployClusterRoleBinding JobType = "clusterrolebinding_deploy"

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
	StatusNotRun         Status = "notRun"                         //没有运行
	StatusPrepare        Status = "prepare"                        //准备
	StatusReject         Status = "reject"                         //拒绝
	StatusDistributed    Status = "distributed"                    //分布式
	StatusWaitingApprove Status = "wait_for_approval"              //等待批准
	StatusDebugBefore    Status = "debug_before"                   //调试开始
	StatusDebugAfter     Status = "debug_after"                    //调试之后
	StatusUnstable       Status = "unstable"                       //不稳定
	StatusManualApproval Status = "wait_for_manual_error_handling" //等待手动错误处理
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
