package service

import (
	v1beta1 "KubeMin-Cli/apis/core.kubemincli.dev/v1alpha1"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WorkflowService interface {
	ListApplicationWorkflow(ctx context.Context, app *model.Applications) error
	SyncWorkflowRecord(ctx context.Context, appKey, recordName string, app *v1beta1.Applications, workflowContext map[string]string) error
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

func (w workflowServiceImpl) ListApplicationWorkflow(ctx context.Context, app *model.Applications) error {
	//TODO implement me
	panic("implement me")
}

func (w workflowServiceImpl) SyncWorkflowRecord(ctx context.Context, appKey, recordName string, app *v1beta1.Applications, workflowContext map[string]string) error {
	//TODO implement me
	panic("implement me")
}
