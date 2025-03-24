package service

import (
	v1beta1 "KubeMin-Cli/apis/core.kubemincli.dev/v1alpha1"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	wf "KubeMin-Cli/pkg/apiserver/workflow"
	job "KubeMin-Cli/pkg/apiserver/workflow/job"
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WorkflowService interface {
	ListApplicationWorkflow(ctx context.Context, app *model.Applications) error
	SyncWorkflowRecord(ctx context.Context, appKey, recordName string, app *v1beta1.Applications, workflowContext map[string]string) error
	CreateWorkflowTask(ctx context.Context, workflow *model.Workflow) error
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
func (w *workflowServiceImpl) CreateWorkflowTask(ctx context.Context, workflow *model.Workflow) error {
	// 校验工作流信息
	if err := wf.LintWorkflow(workflow); err != nil {
		return err
	}

	workflowTask := &model.WorkflowTask{}

	// TODO 查询用户信息,与工作流连接在一起
	originalWorkflow, err := repository.WorkflowByName(ctx, w.Store, workflow.Name)
	if err != nil {
		return fmt.Errorf("cannot find workflow %s, error: %v", workflow.Name, err)
	}

	if workflow.Disabled || originalWorkflow.Disabled {
		return fmt.Errorf("workflow is disabled ")
	}

	workflowTask.Hash = originalWorkflow.Hash

	workflow.CreateTime = time.Now()
	workflow.UpdateTime = time.Now()

	//初始化工作流
	if err := job.InstantiateWorkflow(workflow); err != nil {
		klog.Error("instantiate workflow error: %s", err)
		return err
	}

	return nil
}

func (w *workflowServiceImpl) ListApplicationWorkflow(ctx context.Context, app *model.Applications) error {
	//TODO implement me
	panic("implement me")
}

func (w *workflowServiceImpl) SyncWorkflowRecord(ctx context.Context, appKey, recordName string, app *v1beta1.Applications, workflowContext map[string]string) error {
	//TODO implement me
	panic("implement me")
}
