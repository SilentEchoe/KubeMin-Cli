package service

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"time"

	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	assembler "KubeMin-Cli/pkg/apiserver/interfaces/api/assembler/v1"
	apisv1 "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils"
)

type ApplicationsService interface {
	ListApplications(ctx context.Context, listOptions apisv1.ListApplicationOptions) ([]*apisv1.ApplicationBase, error)
	DeleteApplication(ctx context.Context, app *model.Applications) error
	Deploy(ctx context.Context, req apisv1.ApplicationsDeployRequest) (*apisv1.ApplicationsDeployResponse, error)
}

type applicationsServiceImpl struct {
	Store      datastore.DataStore `inject:"datastore"`
	KubeClient client.Client       `inject:"kubeClient"`
}

func NewApplicationService() ApplicationsService {
	return &applicationsServiceImpl{}
}

// ListApplications list applications
func (c *applicationsServiceImpl) ListApplications(ctx context.Context, listOptions apisv1.ListApplicationOptions) ([]*apisv1.ApplicationBase, error) {
	apps, err := listApp(ctx, c.Store, listOptions)
	if err != nil {
		return nil, err
	}
	var list []*apisv1.ApplicationBase
	for _, app := range apps {
		appBase := assembler.ConvertAppModelToBase(app)
		list = append(list, appBase)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdateTime.Unix() > list[j].UpdateTime.Unix()
	})
	return list, nil

}

func listApp(ctx context.Context, ds datastore.DataStore, listOptions apisv1.ListApplicationOptions) ([]*model.Applications, error) {
	// 这里写的简单一点，直接查询所有的应用列表，后续在做身份认证信息
	var app = model.Applications{}
	var err error
	entities, err := ds.List(ctx, &app, &datastore.ListOptions{})
	if err != nil {
		return nil, err
	}
	var list []*model.Applications

	for _, entity := range entities {
		appModel, ok := entity.(*model.Applications)
		if !ok {
			continue
		}
		list = append(list, appModel)
	}

	return list, nil

}

// DeleteApplication delete application
func (c *applicationsServiceImpl) DeleteApplication(ctx context.Context, app *model.Applications) error {
	return c.Store.Delete(ctx, app)
}

func (c *applicationsServiceImpl) Deploy(ctx context.Context, req apisv1.ApplicationsDeployRequest) (*apisv1.ApplicationsDeployResponse, error) {
	// 根据时间生成一个版本号
	version := utils.GenerateVersion("")
	return &apisv1.ApplicationsDeployResponse{Version: version, CreateTime: time.Now()}, nil
}
