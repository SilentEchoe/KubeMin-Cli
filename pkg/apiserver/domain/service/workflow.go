package service

import (
	v1beta1 "KubeMin-Cli/apis/core.kubemincli.dev/v1alpha1"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"context"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WorkflowService interface {
	ListApplicationWorkflow(ctx context.Context, app *model.Applications) error
	SyncWorkflowRecord(ctx context.Context, appKey, recordName string, app *v1beta1.Applications, workflowContext map[string]string) error
	CreateWorkflowTask(ctx context.Context, workflow apis.CreateWorkflowRequest) (apis.CreateWorkflowResponse, error)
}

type workflowServiceImpl struct {
	Store      datastore.DataStore `inject:"datastore"`
	KubeClient client.Client       `inject:"kubeClient"`
	KubeConfig *rest.Config        `inject:"kubeConfig"`
}

// NewWorkflowService new workflow service
func NewWorkflowService() WorkflowService {
	return &workflowServiceImpl{}
}

// CreateWorkflowTask 创建工作流任务(执行)
func (w *workflowServiceImpl) CreateWorkflowTask(ctx context.Context, workflow apis.CreateWorkflowRequest) (apis.CreateWorkflowResponse, error) {

	//// 校验工作流信息
	//if err := wf.LintWorkflow(workflow); err != nil {
	//	return err
	//}
	//
	////初始化工作流
	//if err := job.InstantiateWorkflow(workflow); err != nil {
	//	klog.Error("instantiate workflow error: %s", err)
	//	return err
	//}
	//
	//return nil
	panic("implement me")
}

func convertWorlflow(req *apis.CreateWorkflowRequest) *model.Workflow {
	return &model.Workflow{}
}

func (w *workflowServiceImpl) ListApplicationWorkflow(ctx context.Context, app *model.Applications) error {
	//TODO implement me
	panic("implement me")
}

func (w *workflowServiceImpl) SyncWorkflowRecord(ctx context.Context, appKey, recordName string, app *v1beta1.Applications, workflowContext map[string]string) error {
	//TODO implement me
	panic("implement me")
}
