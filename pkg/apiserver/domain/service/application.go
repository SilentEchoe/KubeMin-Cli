package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/repository"
	"KubeMin-Cli/pkg/apiserver/event/workflow/job"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	assembler "KubeMin-Cli/pkg/apiserver/interfaces/api/assembler/v1"
	apisv1 "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
	"KubeMin-Cli/pkg/apiserver/workflow/naming"
)

const cleanupTimeout = 10 * time.Second

type ApplicationsService interface {
	CreateApplications(context.Context, apisv1.CreateApplicationsRequest) (*apisv1.ApplicationBase, error)
	GetApplication(ctx context.Context, appName string) (*model.Applications, error)
	ListApplications(ctx context.Context) ([]*apisv1.ApplicationBase, error)
	DeleteApplication(ctx context.Context, app *model.Applications) error
	CleanupApplicationResources(ctx context.Context, appID string) (*apisv1.CleanupApplicationResourcesResponse, error)
	UpdateApplicationWorkflow(ctx context.Context, appID string, req apisv1.UpdateApplicationWorkflowRequest) (*apisv1.UpdateWorkflowResponse, error)
	ListApplicationWorkflows(ctx context.Context, appID string) ([]*model.Workflow, error)
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
		application, err = refreshExistingApplication(ctx, c.Store, req)
		if err != nil {
			return nil, err
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

	// 预清理：删除该应用ID下的所有现有组件，确保幂等性
	if err = repository.DelComponentsByAppID(ctx, c.Store, application.ID); err != nil {
		klog.Errorf("Pre-cleanup components for application %s failed: %v", application.ID, err)
		return nil, bcode.ErrComponentBuild
	}

	// 批量创建组件
	if len(components) > 0 {
		// 转换为Entity接口数组用于批量操作
		entities := make([]datastore.Entity, len(components))
		for i, component := range components {
			entities[i] = component
		}

		// 使用批量创建，失败时回滚
		if err = c.Store.BatchAdd(ctx, entities); err != nil {
			klog.Errorf("Batch create components for application %s failed: %v", application.ID, err)
			// 批量创建失败，清理已创建的部分组件
			if cleanupErr := repository.DelComponentsByAppID(ctx, c.Store, application.ID); cleanupErr != nil {
				klog.Errorf("Cleanup components on failure for application %s failed: %v", application.ID, cleanupErr)
			}
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
		ProjectID:    application.Project,
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

func refreshExistingApplication(ctx context.Context, store datastore.DataStore, req apisv1.CreateApplicationsRequest) (*model.Applications, error) {
	application, err := repository.ApplicationByID(ctx, store, req.ID)
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

	if err = repository.DelComponentsByAppID(ctx, store, req.ID); err != nil {
		return nil, bcode.ErrComponentBuild
	}
	if err = repository.DelWorkflowsByAppID(ctx, store, req.ID); err != nil {
		return nil, bcode.ErrWorkflowBuild
	}
	return application, nil
}

func prepareComponents(appID, namespace string, reqComponents []apisv1.CreateComponentRequest) ([]*model.ApplicationComponent, error) {
	components := make([]*model.ApplicationComponent, 0, len(reqComponents))
	for _, reqComponent := range reqComponents {
		if (reqComponent.ComponentType == config.ServerJob || reqComponent.ComponentType == config.StoreJob) && reqComponent.Image == "" {
			return nil, bcode.ErrComponentNotImageSet
		}

		reqComponent.NameSpace = namespace
		// 复制 ConvertComponent 逻辑，确保一致性
		if reqComponent.Replicas <= 0 {
			reqComponent.Replicas = 1
		}
		component := &model.ApplicationComponent{
			Name:          reqComponent.Name,
			AppID:         appID,
			Namespace:     reqComponent.NameSpace,
			Image:         reqComponent.Image,
			Replicas:      reqComponent.Replicas,
			ComponentType: reqComponent.ComponentType,
		}

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

func ensureUniqueWorkflowName(base string, workflows []*model.Workflow) string {
	if base == "" {
		return base
	}
	candidate := base
	suffix := 1
	for {
		inUse := false
		for _, wf := range workflows {
			if wf == nil {
				continue
			}
			if strings.EqualFold(wf.Name, candidate) {
				inUse = true
				break
			}
		}
		if !inUse {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, suffix)
		suffix++
	}
}

func deriveWorkflowMetadata(app *model.Applications, workflows []*model.Workflow) (namespace, projectID, description string) {
	if app != nil {
		namespace = app.Namespace
		projectID = app.Project
		description = app.Description
	}
	for _, wf := range workflows {
		if wf == nil {
			continue
		}
		if wf.Namespace != "" {
			namespace = wf.Namespace
		}
		if wf.ProjectID != "" {
			projectID = wf.ProjectID
		}
		if wf.Description != "" {
			description = wf.Description
		}
		break
	}
	return
}

func validateWorkflowComponentRefs(steps []apisv1.CreateWorkflowStepRequest, existing map[string]struct{}) error {
	for _, step := range steps {
		if err := ensureComponentsExist(mergeWorkflowComponents(step.Components, step.Properties.Policies), existing); err != nil {
			klog.Errorf("workflow step=%s references missing components: %v", step.Name, err)
			return err
		}
		for _, sub := range step.SubSteps {
			if err := ensureComponentsExist(mergeWorkflowComponents(sub.Components, sub.Properties.Policies), existing); err != nil {
				klog.Errorf("workflow substep=%s references missing components: %v", sub.Name, err)
				return err
			}
		}
	}
	return nil
}

func ensureComponentsExist(names []string, existing map[string]struct{}) error {
	for _, name := range names {
		lower := strings.ToLower(name)
		if _, ok := existing[lower]; !ok {
			return fmt.Errorf("%w: component %q not found", bcode.ErrWorkflowConfig, name)
		}
	}
	return nil
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

func (c *applicationsServiceImpl) CleanupApplicationResources(ctx context.Context, appID string) (*apisv1.CleanupApplicationResourcesResponse, error) {
	if appID == "" {
		return nil, bcode.ErrApplicationNotExist
	}
	app, err := repository.ApplicationByID(ctx, c.Store, appID)
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}
	components, err := repository.FindComponentsByAppID(ctx, c.Store, app.ID)
	if err != nil {
		return nil, err
	}
	if len(components) == 0 {
		return &apisv1.CleanupApplicationResourcesResponse{AppID: app.ID}, nil
	}

	reporter := newCleanupReporter()
	for _, component := range components {
		if component == nil {
			continue
		}
		if err := c.deleteComponentResources(ctx, component, reporter); err != nil {
			klog.Errorf("cleanup component %s/%s failed: %v", component.Name, component.AppID, err)
		}
	}

	resp := &apisv1.CleanupApplicationResourcesResponse{
		AppID:            app.ID,
		DeletedResources: reporter.deletedResources,
	}
	if len(reporter.failedResources) > 0 {
		resp.FailedResources = reporter.failedResources
		return resp, reporter.err()
	}
	return resp, nil
}

func (c *applicationsServiceImpl) deleteComponentResources(ctx context.Context, component *model.ApplicationComponent, reporter *cleanupReporter) error {
	props := job.ParseProperties(component.Properties)
	componentCopy := *component
	if componentCopy.Namespace == "" {
		componentCopy.Namespace = config.DefaultNamespace
	}
	componentPtr := &componentCopy

	switch component.ComponentType {
	case config.ServerJob:
		result := job.GenerateWebService(componentPtr, &props)
		deployNS := componentPtr.Namespace
		deployName := naming.WebServiceName(component.Name, component.AppID)
		if result != nil {
			if deploy, ok := result.Service.(*appsv1.Deployment); ok && deploy != nil {
				if deploy.Namespace != "" {
					deployNS = deploy.Namespace
				}
				if deploy.Name != "" {
					deployName = deploy.Name
				}
			}
			c.deleteAdditionalObjects(ctx, componentPtr.Namespace, result.AdditionalObjects, reporter)
		}
		reporter.record("Deployment", deployNS, deployName, c.deleteDeployment(ctx, deployNS, deployName))
	case config.StoreJob:
		result := job.GenerateStoreService(componentPtr)
		statefulNS := componentPtr.Namespace
		statefulName := naming.StoreServerName(component.Name, component.AppID)
		if result != nil {
			if sts, ok := result.Service.(*appsv1.StatefulSet); ok && sts != nil {
				if sts.Namespace != "" {
					statefulNS = sts.Namespace
				}
				if sts.Name != "" {
					statefulName = sts.Name
				}
			}
			c.deleteAdditionalObjects(ctx, componentPtr.Namespace, result.AdditionalObjects, reporter)
		}
		reporter.record("StatefulSet", statefulNS, statefulName, c.deleteStatefulSet(ctx, statefulNS, statefulName))
	case config.ConfJob:
		c.deleteConfigMapForComponent(ctx, componentPtr, &props, reporter)
	case config.SecretJob:
		c.deleteSecretForComponent(ctx, componentPtr, &props, reporter)
	}

	c.deleteServiceForComponent(ctx, componentPtr, &props, reporter)
	return nil
}

func (c *applicationsServiceImpl) UpdateApplicationWorkflow(ctx context.Context, appID string, req apisv1.UpdateApplicationWorkflowRequest) (*apisv1.UpdateWorkflowResponse, error) {
	if appID == "" {
		return nil, bcode.ErrApplicationNotExist
	}
	if len(req.Workflow) == 0 {
		return nil, bcode.ErrWorkflowConfig
	}
	app, err := repository.ApplicationByID(ctx, c.Store, appID)
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}

	components, err := repository.FindComponentsByAppID(ctx, c.Store, app.ID)
	if err != nil {
		return nil, err
	}
	componentSet := make(map[string]struct{}, len(components))
	for _, comp := range components {
		componentSet[strings.ToLower(comp.Name)] = struct{}{}
	}
	if err := validateWorkflowComponentRefs(req.Workflow, componentSet); err != nil {
		klog.Errorf("component validation failed for app=%s workflowId=%s: %v", appID, req.WorkflowID, err)
		return nil, err
	}

	workflowSteps := convertWorkflowStepsFromRequest(req.Workflow)
	stepsStruct, err := model.NewJSONStructByStruct(workflowSteps)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow steps: %w", err)
	}

	workflows, err := repository.FindWorkflowsByAppID(ctx, c.Store, app.ID)
	if err != nil {
		return nil, err
	}

	targetName := strings.ToLower(strings.TrimSpace(req.Name))
	var target *model.Workflow
	if req.WorkflowID != "" {
		wf, err := repository.WorkflowByID(ctx, c.Store, req.WorkflowID)
		if err != nil {
			if errors.Is(err, datastore.ErrRecordNotExist) {
				return nil, bcode.ErrWorkflowNotExist
			}
			return nil, err
		}
		if wf.AppID != app.ID {
			return nil, bcode.ErrWorkflowConfig
		}
		target = wf
		if targetName == "" {
			targetName = target.Name
		}
	} else {
		if targetName == "" {
			targetName = fmt.Sprintf("%s-workflow", strings.ToLower(app.Name))
		}
		targetName = ensureUniqueWorkflowName(targetName, workflows)
	}

	if target == nil {
		namespace, projectID, description := deriveWorkflowMetadata(app, workflows)
		target = &model.Workflow{
			ID:           utils.RandStringByNumLowercase(24),
			Name:         targetName,
			Alias:        req.Alias,
			Namespace:    namespace,
			Disabled:     false,
			ProjectID:    projectID,
			AppID:        app.ID,
			Description:  description,
			WorkflowType: config.WorkflowTaskTypeWorkflow,
			Status:       config.StatusCreated,
		}
		target.Steps = stepsStruct
		if err := repository.CreateWorkflow(ctx, c.Store, target); err != nil {
			return nil, err
		}
	} else {
		if targetName != "" {
			target.Name = targetName
		}
		if req.Alias != "" {
			target.Alias = req.Alias
		}
		target.Steps = stepsStruct
		if err := c.Store.Put(ctx, target); err != nil {
			return nil, err
		}
	}
	return &apisv1.UpdateWorkflowResponse{WorkflowID: target.ID}, nil
}

func (c *applicationsServiceImpl) ListApplicationWorkflows(ctx context.Context, appID string) ([]*model.Workflow, error) {
	if appID == "" {
		return nil, bcode.ErrApplicationNotExist
	}
	app, err := repository.ApplicationByID(ctx, c.Store, appID)
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}
	workflows, err := repository.FindWorkflowsByAppID(ctx, c.Store, app.ID)
	if err != nil {
		return nil, err
	}
	if len(workflows) == 0 {
		return nil, nil
	}
	sort.SliceStable(workflows, func(i, j int) bool {
		if workflows[i] == nil || workflows[j] == nil {
			return workflows[j] != nil
		}
		return workflows[i].UpdateTime.Unix() > workflows[j].UpdateTime.Unix()
	})
	return workflows, nil
}

func (c *applicationsServiceImpl) deleteServiceForComponent(ctx context.Context, component *model.ApplicationComponent, props *model.Properties, reporter *cleanupReporter) {
	if props == nil || len(props.Ports) == 0 {
		return
	}
	svc := job.GenerateService(component, props)
	ns := component.Namespace
	name := naming.ServiceName(component.Name, component.AppID)
	if svc != nil && svc.Name != nil && *svc.Name != "" {
		name = *svc.Name
		if svc.Namespace != nil && *svc.Namespace != "" {
			ns = *svc.Namespace
		}
	}
	reporter.record("Service", ns, name, c.deleteService(ctx, ns, name))
}

func (c *applicationsServiceImpl) deleteConfigMapForComponent(ctx context.Context, component *model.ApplicationComponent, props *model.Properties, reporter *cleanupReporter) {
	obj := job.GenerateConfigMap(component, props)
	switch cm := obj.(type) {
	case *model.ConfigMapInput:
		ns := pickNamespace(cm.Namespace, component.Namespace)
		name := cm.Name
		if name == "" {
			name = component.Name
		}
		reporter.record("ConfigMap", ns, name, c.deleteConfigMap(ctx, ns, name))
	case *corev1.ConfigMap:
		ns := pickNamespace(cm.Namespace, component.Namespace)
		name := cm.Name
		if name == "" {
			name = component.Name
		}
		reporter.record("ConfigMap", ns, name, c.deleteConfigMap(ctx, ns, name))
	default:
		// nothing to delete
	}
}

func (c *applicationsServiceImpl) deleteSecretForComponent(ctx context.Context, component *model.ApplicationComponent, props *model.Properties, reporter *cleanupReporter) {
	obj := job.GenerateSecret(component, props)
	switch sec := obj.(type) {
	case *model.SecretInput:
		ns := pickNamespace(sec.Namespace, component.Namespace)
		name := sec.Name
		if name == "" {
			name = component.Name
		}
		reporter.record("Secret", ns, name, c.deleteSecret(ctx, ns, name))
	case *corev1.Secret:
		ns := pickNamespace(sec.Namespace, component.Namespace)
		name := sec.Name
		if name == "" {
			name = component.Name
		}
		reporter.record("Secret", ns, name, c.deleteSecret(ctx, ns, name))
	default:
		// nothing
	}
}

func (c *applicationsServiceImpl) deleteAdditionalObjects(ctx context.Context, fallbackNamespace string, objs []client.Object, reporter *cleanupReporter) {
	for _, obj := range objs {
		switch resource := obj.(type) {
		case *corev1.PersistentVolumeClaim:
			ns := pickNamespace(resource.Namespace, fallbackNamespace)
			reporter.record("PersistentVolumeClaim", ns, resource.Name, c.deletePVC(ctx, ns, resource.Name))
		case *networkingv1.Ingress:
			ns := pickNamespace(resource.Namespace, fallbackNamespace)
			reporter.record("Ingress", ns, resource.Name, c.deleteIngress(ctx, ns, resource.Name))
		case *corev1.ServiceAccount:
			ns := pickNamespace(resource.Namespace, fallbackNamespace)
			reporter.record("ServiceAccount", ns, resource.Name, c.deleteServiceAccount(ctx, ns, resource.Name))
		case *rbacv1.Role:
			ns := pickNamespace(resource.Namespace, fallbackNamespace)
			reporter.record("Role", ns, resource.Name, c.deleteRole(ctx, ns, resource.Name))
		case *rbacv1.RoleBinding:
			ns := pickNamespace(resource.Namespace, fallbackNamespace)
			reporter.record("RoleBinding", ns, resource.Name, c.deleteRoleBinding(ctx, ns, resource.Name))
		case *rbacv1.ClusterRole:
			reporter.record("ClusterRole", "", resource.Name, c.deleteClusterRole(ctx, resource.Name))
		case *rbacv1.ClusterRoleBinding:
			reporter.record("ClusterRoleBinding", "", resource.Name, c.deleteClusterRoleBinding(ctx, resource.Name))
		default:
			klog.V(4).Infof("cleanup: unsupported additional object type %T", obj)
		}
	}
}

func (c *applicationsServiceImpl) deleteDeployment(ctx context.Context, namespace, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteNamespaced(ctx, namespace, func(opCtx context.Context, ns string) error {
		return c.KubeClient.AppsV1().Deployments(ns).Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deleteStatefulSet(ctx context.Context, namespace, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteNamespaced(ctx, namespace, func(opCtx context.Context, ns string) error {
		return c.KubeClient.AppsV1().StatefulSets(ns).Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deleteService(ctx context.Context, namespace, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteNamespaced(ctx, namespace, func(opCtx context.Context, ns string) error {
		return c.KubeClient.CoreV1().Services(ns).Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deleteConfigMap(ctx context.Context, namespace, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteNamespaced(ctx, namespace, func(opCtx context.Context, ns string) error {
		return c.KubeClient.CoreV1().ConfigMaps(ns).Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deleteSecret(ctx context.Context, namespace, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteNamespaced(ctx, namespace, func(opCtx context.Context, ns string) error {
		return c.KubeClient.CoreV1().Secrets(ns).Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deletePVC(ctx context.Context, namespace, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteNamespaced(ctx, namespace, func(opCtx context.Context, ns string) error {
		return c.KubeClient.CoreV1().PersistentVolumeClaims(ns).Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deleteIngress(ctx context.Context, namespace, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteNamespaced(ctx, namespace, func(opCtx context.Context, ns string) error {
		return c.KubeClient.NetworkingV1().Ingresses(ns).Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deleteServiceAccount(ctx context.Context, namespace, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteNamespaced(ctx, namespace, func(opCtx context.Context, ns string) error {
		return c.KubeClient.CoreV1().ServiceAccounts(ns).Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deleteRole(ctx context.Context, namespace, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteNamespaced(ctx, namespace, func(opCtx context.Context, ns string) error {
		return c.KubeClient.RbacV1().Roles(ns).Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deleteRoleBinding(ctx context.Context, namespace, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteNamespaced(ctx, namespace, func(opCtx context.Context, ns string) error {
		return c.KubeClient.RbacV1().RoleBindings(ns).Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deleteClusterRole(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteCluster(ctx, func(opCtx context.Context) error {
		return c.KubeClient.RbacV1().ClusterRoles().Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deleteClusterRoleBinding(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	return c.deleteCluster(ctx, func(opCtx context.Context) error {
		return c.KubeClient.RbacV1().ClusterRoleBindings().Delete(opCtx, name, metav1.DeleteOptions{})
	})
}

func (c *applicationsServiceImpl) deleteNamespaced(ctx context.Context, namespace string, fn func(context.Context, string) error) error {
	ns := namespace
	if ns == "" {
		ns = config.DefaultNamespace
	}
	opCtx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()
	err := fn(opCtx, ns)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (c *applicationsServiceImpl) deleteCluster(ctx context.Context, fn func(context.Context) error) error {
	opCtx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()
	err := fn(opCtx)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	return nil
}

type cleanupReporter struct {
	deletedResources []string
	failedResources  []string
	errs             []error
}

func newCleanupReporter() *cleanupReporter {
	return &cleanupReporter{
		deletedResources: []string{},
		failedResources:  []string{},
	}
}

func (r *cleanupReporter) record(kind, namespace, name string, err error) {
	if name == "" {
		return
	}
	target := formatResource(kind, namespace, name)
	if err != nil {
		r.failedResources = append(r.failedResources, fmt.Sprintf("%s (%v)", target, err))
		r.errs = append(r.errs, err)
	} else {
		r.deletedResources = append(r.deletedResources, target)
	}
}

func (r *cleanupReporter) err() error {
	if len(r.errs) == 0 {
		return nil
	}
	if len(r.errs) == 1 {
		return r.errs[0]
	}
	return fmt.Errorf("%d cleanup operations failed; first error: %w", len(r.errs), r.errs[0])
}

func formatResource(kind, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s:%s", kind, name)
	}
	return fmt.Sprintf("%s:%s/%s", kind, namespace, name)
}

func pickNamespace(candidate, fallback string) string {
	if candidate != "" {
		return candidate
	}
	if fallback != "" {
		return fallback
	}
	return config.DefaultNamespace
}

// rollbackApplicationCreation 回滚应用创建过程中的组件创建
func (c *applicationsServiceImpl) rollbackApplicationCreation(ctx context.Context, application *model.Applications) {
	if application == nil {
		return
	}
	if err := repository.DelComponentsByAppID(ctx, c.Store, application.ID); err != nil {
		klog.Errorf("cleanup components for application %s failed: %v", application.ID, err)
	}
}
