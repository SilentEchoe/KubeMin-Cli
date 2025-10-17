package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	assembler "KubeMin-Cli/pkg/apiserver/interfaces/api/assembler/v1"
	apisv1 "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
)

type ApplicationsService interface {
	CreateApplications(context.Context, apisv1.CreateApplicationsRequest) (*apisv1.ApplicationBase, error)
	GetApplication(ctx context.Context, appName string) (*model.Applications, error)
	ListApplications(ctx context.Context) ([]*apisv1.ApplicationBase, error)
	DeleteApplication(ctx context.Context, app *model.Applications) error
}

type applicationsServiceImpl struct {
	Store      datastore.DataStore  `inject:"datastore"`
	KubeClient kubernetes.Interface `inject:"kubeClient"`
}

func NewApplicationService() ApplicationsService {
	return &applicationsServiceImpl{}
}

func (c *applicationsServiceImpl) CreateApplications(ctx context.Context, req apisv1.CreateApplicationsRequest) (*apisv1.ApplicationBase, error) {
	if req.Version == "" {
		req.Version = "1.0.0"
	}

	var (
		application *model.Applications
		err         error
	)

	if req.ID != "" {
		application, err = repository.ApplicationById(ctx, c.Store, req.ID)
		if err != nil {
			return nil, bcode.ErrApplicationNotExist
		}
		application.Name = req.Name
		application.Version = req.Version
		application.Alias = req.Alias
		application.Project = req.Project
		application.Description = req.Description
		application.Icon = req.Icon
		if req.NameSpace != "" {
			application.Namespace = req.NameSpace
		}
		if err = repository.DelComponentsByAppId(ctx, c.Store, req.ID); err != nil {
			return nil, bcode.ErrComponentBuild
		}
		if err = repository.DelWorkflowsByAppId(ctx, c.Store, req.ID); err != nil {
			return nil, bcode.ErrWorkflowBuild
		}
	} else {
		application = model.NewApplications(
			utils.RandStringByNumLowercase(24),
			req.Name,
			req.NameSpace,
			req.Version,
			req.Alias,
			req.Project,
			req.Description,
			req.Icon,
		)
	}
	if application.Namespace == "" {
		application.Namespace = config.DefaultNamespace
	}
	if err = repository.CreateApplications(ctx, c.Store, application); err != nil {
		if errors.Is(err, datastore.ErrRecordExist) {
			return nil, bcode.ErrApplicationExist
		}
		return nil, err
	}

	components, err := prepareComponents(application.ID, application.Namespace, req.Component)
	if err != nil {
		return nil, err
	}

	for _, component := range components {
		if err = repository.CreateComponents(ctx, c.Store, component); err != nil {
			klog.Errorf("Create Components err:%s", err)
			return nil, bcode.ErrCreateComponents
		}
	}

	workflowAliasBase := req.Alias
	if workflowAliasBase == "" {
		workflowAliasBase = req.Name
	}

	workflowAlias := fmt.Sprintf("%s-%s", workflowAliasBase, "workflow")
	workflowName := fmt.Sprintf("%s-%s", req.Name, "workflow")
	var workflowBody interface{}
	if len(req.WorkflowSteps) == 0 {
		workflowBody = convertWorkflowStepByComponent(req.Component)
	} else {
		workflowName = fmt.Sprintf("%s-%s", req.Name, utils.RandStringByNumLowercase(16))
		workflowBody = convertWorkflowStepsFromRequest(req.WorkflowSteps)
	}

	workflowStep, stepErr := model.NewJSONStructByStruct(workflowBody)
	if stepErr != nil {
		return nil, bcode.ErrCreateWorkflow
	}

	workflow := &model.Workflow{
		ID:           utils.RandStringByNumLowercase(24),
		Name:         workflowName,
		Namespace:    application.Namespace,
		AppID:        application.ID,
		Alias:        workflowAlias,
		Disabled:     false,
		ProjectId:    application.Project,
		Description:  application.Description,
		WorkflowType: config.WorkflowTaskTypeWorkflow,
		Status:       config.StatusCreated,
		Steps:        workflowStep,
	}

	if err = repository.CreateWorkflow(ctx, c.Store, workflow); err != nil {
		klog.Errorf("Create workflow err: %v", err)
		return nil, bcode.ErrCreateWorkflow
	}

	base := assembler.ConvertAppModelToBase(application, workflow.ID)
	return base, nil
}

func prepareComponents(appID, namespace string, reqComponents []apisv1.CreateComponentRequest) ([]*model.ApplicationComponent, error) {
	components := make([]*model.ApplicationComponent, 0, len(reqComponents))
	for _, reqComponent := range reqComponents {
		if (reqComponent.ComponentType == config.ServerJob || reqComponent.ComponentType == config.StoreJob) && reqComponent.Image == "" {
			return nil, bcode.ErrComponentNotImageSet
		}

		reqComponent.NameSpace = namespace
		component := ConvertComponent(&reqComponent, appID)

		properties, err := model.NewJSONStructByStruct(reqComponent.Properties)
		if err != nil {
			klog.Errorf("new properties failure,%s", err.Error())
			return nil, bcode.ErrInvalidProperties
		}
		component.Properties = properties

		traits, err := model.NewJSONStructByStruct(reqComponent.Traits)
		if err != nil {
			klog.Errorf("new trait failure,%s", err.Error())
			return nil, bcode.ErrInvalidProperties
		}
		component.Traits = traits

		components = append(components, component)
	}
	return components, nil
}

func convertWorkflowStepByComponent(components []apisv1.CreateComponentRequest) *model.WorkflowSteps {
	workflowSteps := new(model.WorkflowSteps)
	for _, component := range components {
		step := &model.WorkflowStep{
			Name:         component.Name,
			WorkflowType: config.JobDeploy,
			Mode:         config.WorkflowModeStepByStep,
			Properties: []model.Policies{{
				Policies: []string{component.Name},
			}},
		}
		workflowSteps.Steps = append(workflowSteps.Steps, step)
	}
	return workflowSteps
}

func convertWorkflowStepsFromRequest(steps []apisv1.CreateWorkflowStepRequest) *model.WorkflowSteps {
	workflowSteps := new(model.WorkflowSteps)
	for _, reqStep := range steps {
		step := &model.WorkflowStep{
			Name:         reqStep.Name,
			WorkflowType: reqStep.WorkflowType,
			Mode:         config.ParseWorkflowMode(reqStep.Mode),
		}
		componentNames := mergeWorkflowComponents(reqStep.Components, reqStep.Properties.Policies)
		if len(componentNames) > 0 {
			step.Properties = []model.Policies{{Policies: componentNames}}
		}
		for _, subReq := range reqStep.SubSteps {
			subStep := &model.WorkflowSubStep{
				Name:         subReq.Name,
				WorkflowType: subReq.WorkflowType,
			}
			subComponents := mergeWorkflowComponents(subReq.Components, subReq.Properties.Policies)
			if len(subComponents) > 0 {
				subStep.Properties = []model.Policies{{Policies: subComponents}}
			}
			step.SubSteps = append(step.SubSteps, subStep)
		}
		workflowSteps.Steps = append(workflowSteps.Steps, step)
	}
	return workflowSteps
}

func mergeWorkflowComponents(explicit []string, policies []string) []string {
	combined := append([]string{}, explicit...)
	combined = append(combined, policies...)
	return dedupeStrings(combined)
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	var result []string
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, exists := seen[v]; exists {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
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
