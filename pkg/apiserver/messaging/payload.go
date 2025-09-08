package messaging

import (
    "encoding/json"
)

// TaskDispatch is the minimal payload for dispatching a workflow task to a worker.
type TaskDispatch struct {
    TaskID     string `json:"taskId"`
    WorkflowID string `json:"workflowId"`
    ProjectID  string `json:"projectId"`
    AppID      string `json:"appId"`
}

func MarshalTaskDispatch(t TaskDispatch) ([]byte, error) {
    return json.Marshal(t)
}

func UnmarshalTaskDispatch(b []byte) (TaskDispatch, error) {
    var t TaskDispatch
    err := json.Unmarshal(b, &t)
    return t, err
}

