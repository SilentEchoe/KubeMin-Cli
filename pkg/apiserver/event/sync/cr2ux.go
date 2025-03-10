package sync

import (
	v1beta1 "KubeMin-Cli/apis/core.kubemincli.dev/v1alpha1"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"context"
	"fmt"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

// CR2UX provides the Add/Update/Delete method
type CR2UX struct {
	ds                 datastore.DataStore
	cli                client.Client
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

func formatAppComposedName(name, namespace string) string {
	return name + "-" + namespace
}
