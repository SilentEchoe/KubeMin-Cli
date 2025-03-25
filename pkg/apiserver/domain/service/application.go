package service

import (
	v1beta1 "KubeMin-Cli/apis/core.kubemincli.dev/v1alpha1"
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
	"context"
	"errors"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
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
	CreateApplications(context.Context, apisv1.CreateApplicationsRequest) (*apisv1.ApplicationBase, error)
	GetApplication(ctx context.Context, appName string) (*model.Applications, error)
	ListApplications(ctx context.Context) ([]*apisv1.ApplicationBase, error)
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

func (c *applicationsServiceImpl) CreateApplications(ctx context.Context, req apisv1.CreateApplicationsRequest) (*apisv1.ApplicationBase, error) {
	application := model.Applications{
		ID:          utils.RandStringByNumLowercase(24),
		Name:        req.Name,
		Alias:       req.Alias,
		Project:     req.Project,
		Description: req.Description,
		Icon:        req.Icon,
	}
	exist, err := repository.IsExist(ctx, c.Store, req.Name)
	if err != nil {
		return nil, bcode.ErrApplicationExist
	}
	if exist {
		return nil, bcode.ErrApplicationExist
	}
	if err := repository.CreateApplications(ctx, c.Store, &application); err != nil {
		if errors.Is(err, datastore.ErrRecordExist) {
			return nil, bcode.ErrApplicationExist
		}
		return nil, err
	}

	for _, component := range req.Component {
		nComponent := ConvertComponent(&component, application.ID)
		properties, err := model.NewJSONStructByStruct(component.Properties)
		if err != nil {
			klog.Errorf("new trait failure,%s", err.Error())
			return nil, bcode.ErrInvalidProperties
		}
		nComponent.Properties = properties

		err = repository.CreateComponents(ctx, c.Store, nComponent)
		if err != nil {
			klog.Errorf("Create Components err:", err)
			return nil, bcode.ErrCreateComponents
		}
	}

	// render appUtil base info.
	base := assembler.ConvertAppModelToBase(&application)
	return base, nil
}

// ListApplications list applications
func (c *applicationsServiceImpl) ListApplications(ctx context.Context) ([]*apisv1.ApplicationBase, error) {
	listOptions := datastore.ListOptions{
		Page:     0,
		PageSize: 10,
	}

	apps, err := repository.ListApplications(ctx, c.Store, listOptions)
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

// GetApplication get application model
func (c *applicationsServiceImpl) GetApplication(ctx context.Context, appName string) (*model.Applications, error) {
	var app = model.Applications{
		Name: appName,
	}
	if err := c.Store.Get(ctx, &app); err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}
	return &app, nil
}

// DeleteApplication delete application
func (c *applicationsServiceImpl) DeleteApplication(ctx context.Context, app *model.Applications) error {
	return c.Store.Delete(ctx, app)
}

func (c *applicationsServiceImpl) Deploy(ctx context.Context, req apisv1.ApplicationsDeployRequest) (*apisv1.ApplicationsDeployResponse, error) {
	// 根据时间生成一个版本号
	version := utils.GenerateVersion("")
	// 获取app信息
	var app = model.Applications{
		Name: req.Name,
	}
	if err := c.Store.Get(ctx, &app); err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}

	_, err := c.renderApplications(ctx, &app, req.WorkflowName, version)
	if err != nil {
		return nil, fmt.Errorf("failed to render application: %w", err)
	}
	//err = c.KubeClient.Create(ctx, App)
	//if err != nil {
	//	return nil, err
	//}
	return &apisv1.ApplicationsDeployResponse{Version: version, CreateTime: time.Now()}, nil
}

func (c *applicationsServiceImpl) renderApplications(ctx context.Context, appModel *model.Applications, reqWorkflowName, version string) (*v1beta1.Applications, error) {
	//var workflow *model.Workflow
	//var err error
	if reqWorkflowName != "" {
		//TODO 如果请求的工作流不为空，则从数据库中查询对应的工作流

	} else {
		//TODO 如果为空，则使用默认的工作流
	}

	deployAppName := appModel.Name

	labels := make(map[string]string)
	for key, value := range appModel.Labels {
		labels[key] = value
	}

	var application = &v1beta1.Applications{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "core.kubemincli.dev/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployAppName,
			Namespace: appModel.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"deployVersion": version,
				// 发布版本是工作流记录的标识符
				"publishVersion": utils.GenerateVersion(reqWorkflowName),
				"appName":        appModel.Name,
				"appAlias":       appModel.Alias,
			},
		},
	}
	return application, nil
}
