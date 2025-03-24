package config

import "strings"

const (
	REDIS             = "redis"
	TIDB              = "tidb"
	MYSQL             = "mysql"
	DBNAME_KUBEMINCLI = "kubemincli"
)

type JobRunPolicy string
type JobType string
type JobErrorPolicy string

const (
	JobNameRegx  = "^[a-z\u4e00-\u9fa5][a-z0-9\u4e00-\u9fa5-]{0,31}$"
	WorkflowRegx = "^[a-zA-Z0-9-]+$"

	JobBuild  JobType = "build"
	JobDeploy JobType = "deploy"

	DefaultRun    JobRunPolicy = ""
	DefaultNotRun JobRunPolicy = "default_not_run"
	ForceRun      JobRunPolicy = "force_run"
	SkipRun       JobRunPolicy = "skip"

	DefaultJobBuild  JobType = "default_build"
	DefaultJobDeploy JobType = "default_deploy"
)

type Status string

func (s Status) ToLower() Status {
	return Status(strings.ToLower(string(s)))
}

const (
	StatusDisabled       Status = "disabled"                       //已关闭
	StatusCreated        Status = "created"                        // 创建
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
