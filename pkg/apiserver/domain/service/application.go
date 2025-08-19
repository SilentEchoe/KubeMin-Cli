package service

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	assembler "KubeMin-Cli/pkg/apiserver/interfaces/api/assembler/v1"
	apisv1 "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
	"context"
	"errors"
	"fmt"
	"sort"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type ApplicationsService interface {
	CreateApplications(context.Context, apisv1.CreateApplicationsRequest) (*apisv1.ApplicationBase, error)
	GetApplication(ctx context.Context, appName string) (*model.Applications, error)
	ListApplications(ctx context.Context) ([]*apisv1.ApplicationBase, error)
	DeleteApplication(ctx context.Context, app *model.Applications) error
}

type applicationsServiceImpl struct {
	Store      datastore.DataStore   `inject:"datastore"`
	KubeClient *kubernetes.Clientset `inject:"kubeClient"`
}

func NewApplicationService() ApplicationsService {
	return &applicationsServiceImpl{}
}

func (c *applicationsServiceImpl) CreateApplications(ctx context.Context, req apisv1.CreateApplicationsRequest) (*apisv1.ApplicationBase, error) {
	if req.Version == "" {
		req.Version = "1.0.0"
	}
	application := model.Applications{
		ID:          utils.RandStringByNumLowercase(24),
		Name:        req.Name,
		Alias:       req.Alias,
		Project:     req.Project,
		Version:     req.Version,
		Description: req.Description,
		Icon:        req.Icon,
	}
	exist, err := repository.IsExist(ctx, c.Store, req.Name, req.Version)
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
		// Unify image source: prefer Properties.Image; fallback to top-level Image.
		if component.Properties.Image == "" && component.Image != "" {
			component.Properties.Image = component.Image
		}
		nComponent := ConvertComponent(&component, application.ID)
		properties, err := model.NewJSONStructByStruct(component.Properties)
		if err != nil {
			klog.Errorf("new properties failure,%s", err.Error())
			return nil, bcode.ErrInvalidProperties
		}
		nComponent.Properties = properties

		//附加特性
		traits, err := model.NewJSONStructByStruct(component.Traits)
		if err != nil {
			klog.Errorf("new trait failure,%s", err.Error())
			return nil, bcode.ErrInvalidProperties
		}

		nComponent.Traits = traits
		err = repository.CreateComponents(ctx, c.Store, nComponent)
		if err != nil {
			klog.Errorf("Create Components err:%s", err)
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
		klog.Errorf("Create workflow err: %v", err)
		return nil, bcode.ErrCreateWorkflow
	}
	base := assembler.ConvertAppModelToBase(&application, workflow.ID)
	return base, nil
}

func ConvertWorkflowStepByComponent(components []apisv1.CreateComponentRequest) *model.WorkflowSteps {
	workflowSteps := new(model.WorkflowSteps)
	for _, component := range components {
		step := &model.WorkflowStep{
			Name:         component.Name,
			WorkflowType: config.JobDeploy,
			Properties: []model.Policies{{
				Policies: []string{component.Name},
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
		// 这里应该是个WorkflowIds
		appBase := assembler.ConvertAppModelToBase(app, "")
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
