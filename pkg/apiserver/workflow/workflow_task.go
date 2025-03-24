package workflow

import "KubeMin-Cli/pkg/apiserver/domain/model"

type CreateTaskResp struct {
	ProjectName  string `json:"project_name"`
	WorkflowName string `json:"workflow_name"`
	TaskID       int64  `json:"task_id"`
}

func CreateWorkflowTask(triggerName string, args *model.Workflow) (*CreateTaskResp, error) {
	//resp := &CreateTaskResp{
	//	ProjectName:  args.Project,
	//	WorkflowName: args.Name,
	//}
	return nil, nil
}
