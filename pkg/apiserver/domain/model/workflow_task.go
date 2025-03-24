package model

import "KubeMin-Cli/pkg/apiserver/config"

type WorkflowTask struct {
	ID           int64        `json:"id,omitempty"`
	TaskID       int64        `json:"task_id"`
	WorkflowName string       `json:"workflow_name"`
	Stages       []*StageTask `bson:"stages"                    json:"stages"`
	Hash         string       `bson:"hash"                      json:"hash"`
}

type StageTask struct {
	Name      string        `bson:"name"            json:"name"`
	Status    config.Status `bson:"status"          json:"status"`
	StartTime int64         `bson:"start_time"      json:"start_time,omitempty"`
	EndTime   int64         `bson:"end_time"        json:"end_time,omitempty"`
	Parallel  bool          `bson:"parallel"        json:"parallel,omitempty"` //是否并发执行
	Jobs      []*JobTask    `bson:"jobs"            json:"jobs,omitempty"`
	Error     string        `bson:"error"           json:"error"`
}
