package sync

import (
	v1beta1 "KubeMin-Cli/apis/core.kubemincli.dev/v1alpha1"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

// CR2UX provides the Add/Update/Delete method
type CR2UX struct {
	ds                 datastore.DataStore
	cli                client.Client
	cache              sync.Map
	applicationService service.ApplicationService
}

func (c *CR2UX) syncAppCreate(targetApp *v1beta1.Applications) error {
	//从Lab中获取APP的名称
	appPrimaryKey := targetApp.Annotations["core.kubemincli.dev/appName"]
	if appPrimaryKey == "" {
		return fmt.Errorf("appName is empty in applications %s", targetApp.Name)
	}
	return nil
}
