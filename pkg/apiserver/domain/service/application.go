package service

import (
	v1beta1 "KubeMin-Cli/apis/core.kubemincli.dev/v1alpha1"
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
	"context"
	"errors"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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
	Store      datastore.DataStore   `inject:"datastore"`
	KubeClient *kubernetes.Clientset `inject:"kubeClient"`
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
	// 创建App组件
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

	// 如果没有定义工作流，默认会自动按照组件和运维特征数组的顺序进行部署，并把Paas服务所在的当前集群作为目标集群。
	workflowName := ""
	workflowAlias := fmt.Sprintf("%s-%s", req.Alias, "default-workflow")
	var workflowStep *model.JSONStruct
	if len(req.WorkflowSteps) == 0 {
		workflowName = fmt.Sprintf("%s-%s", req.Name, "default-workflow")
		step := ConvertWorkflowStepByComponent(req.Component)
		workflowStep, err = model.NewJSONStructByStruct(step)
		if err != nil {
			return nil, bcode.ErrCreateWorkflow
		}
	} else {
		workflowName = fmt.Sprintf("%s-%s", req.Name, utils.RandStringByNumLowercase(16))
		workflowSteps := new(model.WorkflowSteps)
		for _, steps := range req.WorkflowSteps {
			step := &model.WorkflowStep{
				Name:         steps.Name,
				WorkflowType: steps.WorkflowType,
				Properties: []model.Policies{
					{Policies: steps.Properties.Policies},
				},
			}
			workflowSteps.Steps = append(workflowSteps.Steps, step)
		}
		workflowStep, err = model.NewJSONStructByStruct(workflowSteps)
		if err != nil {
			return nil, bcode.ErrCreateWorkflow
		}
	}
	workflow := &model.Workflow{
		ID:           utils.RandStringByNumLowercase(24),
		Name:         workflowName,
		AppID:        application.ID,
		Alias:        workflowAlias,
		Disabled:     false,
		ProjectId:    application.Project,
		Description:  application.Description,
		WorkflowType: config.WorkflowTaskTypeWorkflow,
		Status:       config.StatusCreated,
		Steps:        workflowStep,
	}

	err = repository.CreateWorkflow(ctx, c.Store, workflow)
	if err != nil {
		klog.Errorf("Create workflow err:", err)
		return nil, bcode.ErrCreateWorkflow
	}
	base := assembler.ConvertAppModelToBase(&application)
	return base, nil
}

func ConvertWorkflowStepByComponent(components []apisv1.CreateComponentRequest) *model.WorkflowSteps {
	workflowSteps := new(model.WorkflowSteps)
	for _, compoent := range components {
		step := &model.WorkflowStep{
			Name:         compoent.Name,
			WorkflowType: config.JobDeploy, //默认部署所有组件
			Properties: []model.Policies{{
				Policies: []string{compoent.Name},
			}},
		}
		workflowSteps.Steps = append(workflowSteps.Steps, step)
	}
	return workflowSteps
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
