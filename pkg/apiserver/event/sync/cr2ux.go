package sync

import (
	v1beta1 "KubeMin-Cli/apis/core.kubemincli.dev/v1alpha1"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// CR2UX provides the Add/Update/Delete method
type CR2UX struct {
	ds                 datastore.DataStore
	cli                *kubernetes.Clientset
	cache              sync.Map
	applicationService service.ApplicationsService
	workflowService    service.WorkflowService
}

func (c *CR2UX) syncAppCreate(targetApp *v1beta1.Applications) error {
	//从Lab中获取APP的名称
	appPrimaryKey := targetApp.Annotations["core.kubemincli.dev/appName"]
	if appPrimaryKey == "" {
		return fmt.Errorf("appName is empty in applications %s", targetApp.Name)
	}
	return nil
}

// AddOrUpdate will sync application CR to storage of VelaUX automatically
func (c *CR2UX) AddOrUpdate(ctx context.Context, targetApp *v1beta1.Applications) error {
	appPrimaryKey := targetApp.Annotations[""]
	if appPrimaryKey == "" {
		return fmt.Errorf("appName is empty in application %s", targetApp.Name)
	}
	var recordName string
	// 同步工作流信息
	if err := c.workflowService.SyncWorkflowRecord(ctx, appPrimaryKey, recordName, targetApp, nil); err != nil {
		klog.ErrorS(err, "failed to sync workflow status", "app name", targetApp.Name, "workflow name", "record name", recordName)
		return err
	}
	return nil
}

// DeleteApp will delete the application as the CR was deleted
func (c *CR2UX) DeleteApp(ctx context.Context, targetApp *v1beta1.Applications) error {
	app, appName, err := c.getApp(ctx, targetApp.Name, targetApp.Namespace)
	if err != nil {
		return err
	}
	// Only for the unit test scenario
	if c.applicationService == nil {
		return c.ds.Delete(ctx, &model.Applications{Name: appName})
	}
	return c.applicationService.DeleteApplication(ctx, app)
}

// getApp will return the app and appname if exists
func (c *CR2UX) getApp(ctx context.Context, name, namespace string) (*model.Applications, string, error) {
	alreadyCreated := &model.Applications{Name: formatAppComposedName(name, namespace)}
	err1 := c.ds.Get(ctx, alreadyCreated)
	if err1 == nil {
		return alreadyCreated, alreadyCreated.Name, nil
	}

	// check if it's created the first in database
	existApp := &model.Applications{Name: name}
	err2 := c.ds.Get(ctx, existApp)
	if err2 == nil {
		en := existApp.Labels[model.LabelSyncNamespace]
		// it means the namespace/app is not created yet, the appname is occupied by app from other namespace
		if en != namespace {
			return nil, formatAppComposedName(name, namespace), err1
		}
		return existApp, name, nil
	}
	return nil, name, err2
}

func formatAppComposedName(name, namespace string) string {
	return name + "-" + namespace
}
