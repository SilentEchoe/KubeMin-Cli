package service

import (
	"context"
	"encoding/json"
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
	ListTemplateApplications(ctx context.Context) ([]*apisv1.ApplicationBase, error)
	DeleteApplication(ctx context.Context, app *model.Applications) error
	CleanupApplicationResources(ctx context.Context, appID string) (*apisv1.CleanupApplicationResourcesResponse, error)
	UpdateApplicationWorkflow(ctx context.Context, appID string, req apisv1.UpdateApplicationWorkflowRequest) (*apisv1.UpdateWorkflowResponse, error)
	ListApplicationWorkflows(ctx context.Context, appID string) ([]*model.Workflow, error)
	ListApplicationComponents(ctx context.Context, appID string) ([]*model.ApplicationComponent, error)
	// UpdateVersion 更新应用版本，支持组件的更新、新增、删除操作
	UpdateVersion(ctx context.Context, appID string, req apisv1.UpdateVersionRequest) (*apisv1.UpdateVersionResponse, error)
}

type applicationsServiceImpl struct {
	KubeClient        kubernetes.Interface               `inject:"kubeClient"`
	Store             datastore.DataStore                `inject:"datastore"`
	AppRepo           repository.ApplicationRepository   `inject:""`
	WorkflowRepo      repository.WorkflowRepository      `inject:""`
	ComponentRepo     repository.ComponentRepository     `inject:""`
	WorkflowQueueRepo repository.WorkflowQueueRepository `inject:""`
}

type componentOverride struct {
	name       string
	compType   config.JobType
	properties apisv1.Properties
	target     string
}

type templateRequest struct {
	baseName  string
	overrides []componentOverride
}

func lastSegment(name string) string {
	parts := strings.Split(name, "-")
	return parts[len(parts)-1]
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

	tmpEnable := false
	if req.TmpEnable != nil {
		tmpEnable = *req.TmpEnable
	}

	if req.ID != "" {
		application, err = c.refreshExistingApplication(ctx, req)
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
			tmpEnable,
		)
	}
	if application.Namespace == "" {
		application.Namespace = config.DefaultNamespace
	}

	//分解所有的组件
	resolvedComponents, err := c.resolveComponents(ctx, application.Namespace, application.Name, req.Component)
	if err != nil {
		return nil, err
	}

	components, err := prepareComponents(application.ID, application.Namespace, resolvedComponents)
	if err != nil {
		return nil, err
	}

	if c.Store == nil {
		return nil, fmt.Errorf("datastore is not initialized")
	}

	var workflow *model.Workflow
	run := func(store datastore.DataStore) error {
		if err := repository.CreateApplications(ctx, store, application); err != nil {
			return err
		}
		if err := repository.DelComponentsByAppID(ctx, store, application.ID); err != nil {
			klog.Errorf("pre-cleanup components for application %s failed: %v", application.ID, err)
			return bcode.ErrComponentBuild
		}
		if err := batchAddComponents(ctx, store, components); err != nil {
			klog.Errorf("batch create components for application %s failed: %v", application.ID, err)
			return bcode.ErrCreateComponents
		}
		wf, err := c.upsertDefaultWorkflow(ctx, store, application, req, resolvedComponents)
		if err != nil {
			return err
		}
		workflow = wf
		return nil
	}

	if tx, ok := c.Store.(datastore.Transactional); ok {
		if err := tx.WithTransaction(ctx, run); err != nil {
			return nil, err
		}
	} else {
		if err := run(c.Store); err != nil {
			return nil, err
		}
	}

	base := assembler.ConvertAppModelToBase(application, workflow.ID)
	return base, nil
}

func batchAddComponents(ctx context.Context, store datastore.DataStore, components []*model.ApplicationComponent) error {
	if len(components) == 0 {
		return nil
	}
	entities := make([]datastore.Entity, len(components))
	for i, comp := range components {
		entities[i] = comp
	}
	return store.BatchAdd(ctx, entities)
}

func (c *applicationsServiceImpl) upsertDefaultWorkflow(ctx context.Context, store datastore.DataStore, app *model.Applications, req apisv1.CreateApplicationsRequest, resolvedComponents []apisv1.CreateComponentRequest) (*model.Workflow, error) {
	workflowAliasBase := req.Alias
	if workflowAliasBase == "" {
		workflowAliasBase = req.Name
	}

	desiredAlias := fmt.Sprintf("%s-workflow", workflowAliasBase)
	desiredName := fmt.Sprintf("%s-workflow", req.Name)

	var workflowBody interface{}
	if len(req.WorkflowSteps) == 0 {
		workflowBody = convertWorkflowStepByComponent(resolvedComponents)
	} else {
		workflowBody = convertWorkflowStepsFromRequest(req.WorkflowSteps)
	}

	workflowStep, err := model.NewJSONStructByStruct(workflowBody)
	if err != nil {
		return nil, bcode.ErrCreateWorkflow
	}

	workflows, err := repository.FindWorkflowsByAppID(ctx, store, app.ID)
	if err != nil {
		return nil, err
	}

	target := pickDefaultWorkflow(workflows, desiredName, desiredAlias)
	if target == nil {
		workflow := &model.Workflow{
			ID:           utils.RandStringByNumLowercase(24),
			Name:         ensureUniqueWorkflowName(desiredName, workflows),
			Namespace:    app.Namespace,
			AppID:        app.ID,
			Alias:        desiredAlias,
			Disabled:     false,
			ProjectID:    app.Project,
			Description:  app.Description,
			WorkflowType: config.WorkflowTaskTypeWorkflow,
			Status:       config.StatusCreated,
			Steps:        workflowStep,
		}
		if err := repository.CreateWorkflow(ctx, store, workflow); err != nil {
			klog.Errorf("create workflow failed: %v", err)
			return nil, bcode.ErrCreateWorkflow
		}
		return workflow, nil
	}

	if desiredName != "" && !strings.EqualFold(target.Name, desiredName) {
		target.Name = ensureUniqueWorkflowNameExcluding(desiredName, workflows, target.ID)
	}
	if desiredAlias != "" {
		target.Alias = desiredAlias
	}
	target.Namespace = app.Namespace
	target.ProjectID = app.Project
	target.Description = app.Description
	target.Steps = workflowStep

	if err := store.Put(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}

func pickDefaultWorkflow(workflows []*model.Workflow, desiredName, desiredAlias string) *model.Workflow {
	best, bestRank := (*model.Workflow)(nil), -1
	for _, wf := range workflows {
		if wf == nil {
			continue
		}
		if wf.WorkflowType != "" && wf.WorkflowType != config.WorkflowTaskTypeWorkflow {
			continue
		}
		rank := 0
		if desiredName != "" && strings.EqualFold(wf.Name, desiredName) {
			rank = 2
		} else if desiredAlias != "" && strings.EqualFold(wf.Alias, desiredAlias) {
			rank = 1
		}
		if best == nil || rank > bestRank ||
			(rank == bestRank && wf.UpdateTime.After(best.UpdateTime)) ||
			(rank == bestRank && wf.UpdateTime.Equal(best.UpdateTime) && wf.CreateTime.After(best.CreateTime)) {
			best = wf
			bestRank = rank
		}
	}
	return best
}

func ensureUniqueWorkflowNameExcluding(base string, workflows []*model.Workflow, excludeID string) string {
	if base == "" {
		return base
	}
	candidate := base
	suffix := 1
	for {
		inUse := false
		for _, wf := range workflows {
			if wf == nil || wf.ID == excludeID {
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

func (c *applicationsServiceImpl) resolveComponents(ctx context.Context, namespace, appName string, reqComponents []apisv1.CreateComponentRequest) ([]apisv1.CreateComponentRequest, error) {
	components := make([]apisv1.CreateComponentRequest, 0, len(reqComponents))
	templateOrder := make([]string, 0)
	templateMap := make(map[string]*templateRequest)

	for _, comp := range reqComponents {
		// 如果该组件没有使用模版，就直接组装这个模版
		if comp.Template == nil || strings.TrimSpace(comp.Template.ID) == "" {
			components = append(components, comp)
			continue
		}
		// 以模板 ID 作为键，从 templateMap 中取出该模板的聚合请求，没有则新建 templateRequest 并记录到 templateOrder，用于后续按出现顺序处理。
		templateID := strings.TrimSpace(comp.Template.ID)
		tr, ok := templateMap[templateID]
		if !ok {
			tr = &templateRequest{baseName: strings.TrimSpace(appName)}
			templateMap[templateID] = tr
			templateOrder = append(templateOrder, templateID)
		}
		if tr.baseName == "" {
			tr.baseName = strings.TrimSpace(appName)
		}
		// 记录需要覆盖的组件名、类型和目标模板组件
		tr.overrides = append(tr.overrides, componentOverride{
			name:       strings.TrimSpace(comp.Name),
			compType:   comp.ComponentType,
			properties: comp.Properties,
			target:     strings.TrimSpace(comp.Template.Target),
		})
	}

	for _, templateID := range templateOrder {
		tr := templateMap[templateID]
		// 克隆
		clones, err := c.cloneComponentsFromTemplate(ctx, namespace, templateID, tr)
		if err != nil {
			return nil, err
		}
		components = append(components, clones...)
	}
	return components, nil
}

func (c *applicationsServiceImpl) cloneComponentsFromTemplate(ctx context.Context, namespace, templateID string, tr *templateRequest) ([]apisv1.CreateComponentRequest, error) {
	if tr == nil {
		return nil, nil
	}
	if templateID == "" {
		return nil, bcode.ErrTemplateIDMissing
	}

	templateApp, err := c.AppRepo.FindByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}
	if !templateApp.TmpEnable {
		return nil, bcode.ErrTemplateNotEnabled
	}

	templateComponents, err := c.ComponentRepo.FindByAppID(ctx, templateApp.ID)
	if err != nil {
		return nil, err
	}
	if len(templateComponents) == 0 {
		return nil, bcode.ErrTemplateComponentMissing
	}

	baseName := strings.TrimSpace(tr.baseName)
	if baseName == "" {
		baseName = templateApp.Name
	}

	type ovState struct {
		componentOverride
		used bool
	}
	overrides := make([]ovState, len(tr.overrides))
	for i, o := range tr.overrides {
		overrides[i] = ovState{componentOverride: o}
	}

	pickOverride := func(templateName string, jobType config.JobType) (*componentOverride, error) {
		for i := range overrides {
			if overrides[i].used {
				continue
			}
			if overrides[i].target != "" && overrides[i].target == templateName {
				if overrides[i].compType != "" && overrides[i].compType != jobType {
					return nil, bcode.ErrTemplateTargetNotFound
				}
				overrides[i].used = true
				return &overrides[i].componentOverride, nil
			}
		}
		for i := range overrides {
			if overrides[i].used {
				continue
			}
			if overrides[i].compType != "" && overrides[i].compType != jobType {
				continue
			}
			overrides[i].used = true
			return &overrides[i].componentOverride, nil
		}
		return nil, nil
	}

	type compPlan struct {
		templateComp *model.ApplicationComponent
		targetName   string
		override     apisv1.Properties
	}
	nameMap := make(map[string]string)
	typeNameMap := make(map[config.JobType]string)
	plans := make([]compPlan, 0, len(templateComponents))
	for _, templateComp := range templateComponents {
		if templateComp == nil {
			continue
		}
		override, err := pickOverride(templateComp.Name, templateComp.ComponentType)
		if err != nil {
			return nil, err
		}

		targetName := templateComp.Name
		if override != nil && override.name != "" {
			targetName = override.name
		} else if baseName != "" {
			targetName = fmt.Sprintf("%s-%s", baseName, lastSegment(templateComp.Name))
		}

		var overrideProps apisv1.Properties
		if override != nil {
			overrideProps = override.properties
		}
		nameMap[templateComp.Name] = targetName
		nameMap[fmt.Sprintf("tem-%s", templateComp.Name)] = targetName
		if _, ok := typeNameMap[templateComp.ComponentType]; !ok {
			typeNameMap[templateComp.ComponentType] = targetName
		}
		plans = append(plans, compPlan{templateComp: templateComp, targetName: targetName, override: overrideProps})
	}

	clones := make([]apisv1.CreateComponentRequest, 0, len(plans))
	for _, plan := range plans {
		clone, err := convertComponentFromTemplate(plan.templateComp, plan.targetName, baseName, namespace, plan.override, nameMap, typeNameMap)
		if err != nil {
			return nil, err
		}
		clones = append(clones, *clone)
	}
	for _, ov := range overrides {
		if ov.used {
			continue
		}
		if ov.target != "" {
			return nil, bcode.ErrTemplateTargetNotFound
		}
	}
	return clones, nil
}

func convertComponentFromTemplate(templateComp *model.ApplicationComponent, newName, baseName, namespace string, overrideProps apisv1.Properties, nameMap map[string]string, typeNameMap map[config.JobType]string) (*apisv1.CreateComponentRequest, error) {
	var properties apisv1.Properties
	if err := decodeJSONStruct(templateComp.Properties, &properties); err != nil {
		return nil, fmt.Errorf("convert template component %s properties: %w", templateComp.Name, err)
	}
	var traits apisv1.Traits
	if err := decodeJSONStruct(templateComp.Traits, &traits); err != nil {
		return nil, fmt.Errorf("convert template component %s traits: %w", templateComp.Name, err)
	}

	applyPropertyOverrides(&properties, overrideProps, templateComp.ComponentType)
	rewriteTraitsForTemplate(&traits, templateComp.Name, newName, baseName, namespace, nameMap, typeNameMap)

	return &apisv1.CreateComponentRequest{
		Name:          newName,
		ComponentType: templateComp.ComponentType,
		Image:         templateComp.Image,
		NameSpace:     namespace,
		Replicas:      templateComp.Replicas,
		Properties:    properties,
		Traits:        traits,
	}, nil
}

func decodeJSONStruct(raw *model.JSONStruct, target interface{}) error {
	if raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	if string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, target)
}

func applyPropertyOverrides(props *apisv1.Properties, override apisv1.Properties, compType config.JobType) {
	if len(override.Env) > 0 {
		if props.Env == nil {
			props.Env = make(map[string]string, len(override.Env))
		}
		for k, v := range override.Env {
			props.Env[k] = v
		}
	}
	if len(override.Secret) > 0 && compType == config.SecretJob {
		if props.Secret == nil {
			props.Secret = make(map[string]string, len(override.Secret))
		}
		for k, v := range override.Secret {
			props.Secret[k] = v
		}
	}
}

// rewriteTraitsForTemplate 通过模版重写特征等元数据
func rewriteTraitsForTemplate(traits *apisv1.Traits, oldName, newName, baseName, namespace string, nameMap map[string]string, typeNameMap map[config.JobType]string) {
	if traits == nil {
		return
	}

	rewriteNameCandidate := func(name string) string {
		if nameMap != nil {
			if mapped, ok := nameMap[name]; ok {
				return mapped
			}
		}
		if name == "" {
			return name
		}
		if strings.Contains(name, oldName) {
			return strings.ReplaceAll(name, oldName, newName)
		}
		return name
	}

	volumePrefix := strings.TrimSpace(baseName)
	if volumePrefix == "" {
		volumePrefix = newName
	}

	for i := range traits.Storage {
		storage := &traits.Storage[i]
		if storage.Type == config.StorageTypePersistent {
			switch {
			case storage.Name == "" || storage.Name == oldName:
				storage.Name = newName
			case strings.Contains(storage.Name, oldName):
				storage.Name = strings.ReplaceAll(storage.Name, oldName, newName)
			case volumePrefix != "" && !(strings.HasPrefix(storage.Name, volumePrefix+"-") || storage.Name == volumePrefix):
				storage.Name = fmt.Sprintf("%s-%s", volumePrefix, storage.Name)
			}
		} else {
			if storage.Name == "" || storage.Name == oldName {
				storage.Name = newName
			} else {
				storage.Name = rewriteNameCandidate(storage.Name)
			}
		}
		if storage.ClaimName != "" {
			storage.ClaimName = rewriteNameCandidate(storage.ClaimName)
		}
		if storage.SourceName != "" {
			storage.SourceName = rewriteNameCandidate(storage.SourceName)
		}
	}

	for i := range traits.Ingress {
		ingress := &traits.Ingress[i]
		if ingress.Name == "" || ingress.Name == oldName {
			ingress.Name = newName
		}
		if ingress.Namespace == "" {
			ingress.Namespace = namespace
		}
		for j := range ingress.Routes {
			if ingress.Routes[j].Backend.ServiceName == oldName {
				ingress.Routes[j].Backend.ServiceName = newName
			}
		}
	}

	for i := range traits.RBAC {
		policy := &traits.RBAC[i]
		// RBAC 资源保持名称不变，但命名空间与组件命名空间对齐（为空则用默认命名空间）。
		if namespace != "" {
			policy.Namespace = namespace
		} else if policy.Namespace == "" {
			policy.Namespace = config.DefaultNamespace
		}
	}

	for i := range traits.EnvFrom {
		traits.EnvFrom[i].SourceName = rewriteNameCandidate(traits.EnvFrom[i].SourceName)
	}

	for i := range traits.Envs {
		env := &traits.Envs[i]
		if env.ValueFrom.Secret != nil {
			env.ValueFrom.Secret.Name = rewriteNameCandidate(env.ValueFrom.Secret.Name)
		}
		if env.ValueFrom.Config != nil {
			env.ValueFrom.Config.Name = rewriteNameCandidate(env.ValueFrom.Config.Name)
		}
	}

	for i := range traits.Init {
		initTrait := &traits.Init[i]
		if initTrait.Name == "" || initTrait.Name == oldName {
			initTrait.Name = fmt.Sprintf("%s-init-%d", newName, i+1)
		}
		rewriteTraitsForTemplate(&initTrait.Traits, oldName, newName, baseName, namespace, nameMap, typeNameMap)
	}

	for i := range traits.Sidecar {
		sidecar := &traits.Sidecar[i]
		if sidecar.Name == "" || sidecar.Name == oldName {
			sidecar.Name = fmt.Sprintf("%s-sidecar-%d", newName, i+1)
		}
		rewriteTraitsForTemplate(&sidecar.Traits, oldName, newName, baseName, namespace, nameMap, typeNameMap)
	}
}

func (c *applicationsServiceImpl) refreshExistingApplication(ctx context.Context, req apisv1.CreateApplicationsRequest) (*model.Applications, error) {
	application, err := c.AppRepo.FindByID(ctx, req.ID)
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
	if req.TmpEnable != nil {
		application.TmpEnable = *req.TmpEnable
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

	apps, err := c.AppRepo.List(ctx, listOptions)
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

// ListTemplateApplications lists applications marked as templates (tmp_enable=true)
func (c *applicationsServiceImpl) ListTemplateApplications(ctx context.Context) ([]*apisv1.ApplicationBase, error) {
	listOptions := datastore.ListOptions{
		Page:     0,
		PageSize: 0, // no pagination to ensure all templates returned
	}

	apps, err := c.AppRepo.List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	var list []*apisv1.ApplicationBase
	for _, app := range apps {
		if app == nil || !app.TmpEnable {
			continue
		}
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
	app, err := c.AppRepo.FindByName(ctx, appName)
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}
	return app, nil
}

// DeleteApplication delete application
func (c *applicationsServiceImpl) DeleteApplication(ctx context.Context, app *model.Applications) error {
	return c.AppRepo.Delete(ctx, app)
}

func (c *applicationsServiceImpl) CleanupApplicationResources(ctx context.Context, appID string) (*apisv1.CleanupApplicationResourcesResponse, error) {
	if appID == "" {
		return nil, bcode.ErrApplicationNotExist
	}
	app, err := c.AppRepo.FindByID(ctx, appID)
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}
	components, err := c.ComponentRepo.FindByAppID(ctx, app.ID)
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
	traits := job.ParseTraits(component.Traits)
	if traits.Bundle != nil && strings.TrimSpace(traits.Bundle.Name) != "" {
		klog.V(4).Infof("skip cleanup for shared bundle component %s/%s (bundle=%s)", component.Name, component.AppID, traits.Bundle.Name)
		return nil
	}
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
	app, err := c.AppRepo.FindByID(ctx, appID)
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}

	components, err := c.ComponentRepo.FindByAppID(ctx, app.ID)
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

	workflows, err := c.WorkflowRepo.FindByAppID(ctx, app.ID)
	if err != nil {
		return nil, err
	}

	targetName := strings.ToLower(strings.TrimSpace(req.Name))
	var target *model.Workflow
	if req.WorkflowID != "" {
		wf, err := c.WorkflowRepo.FindByID(ctx, req.WorkflowID)
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
		if err := c.WorkflowRepo.Create(ctx, target); err != nil {
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
		if err := c.WorkflowRepo.Update(ctx, target); err != nil {
			return nil, err
		}
	}
	return &apisv1.UpdateWorkflowResponse{WorkflowID: target.ID}, nil
}

func (c *applicationsServiceImpl) ListApplicationComponents(ctx context.Context, appID string) ([]*model.ApplicationComponent, error) {
	if appID == "" {
		return nil, bcode.ErrApplicationNotExist
	}
	app, err := c.AppRepo.FindByID(ctx, appID)
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}
	components, err := c.ComponentRepo.FindByAppID(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	if len(components) == 0 {
		return nil, nil
	}
	sort.SliceStable(components, func(i, j int) bool {
		if components[i] == nil || components[j] == nil {
			return components[j] != nil
		}
		return components[i].UpdateTime.Unix() > components[j].UpdateTime.Unix()
	})
	return components, nil
}

func (c *applicationsServiceImpl) ListApplicationWorkflows(ctx context.Context, appID string) ([]*model.Workflow, error) {
	if appID == "" {
		return nil, bcode.ErrApplicationNotExist
	}
	app, err := c.AppRepo.FindByID(ctx, appID)
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}
	workflows, err := c.WorkflowRepo.FindByAppID(ctx, app.ID)
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
	if err := c.ComponentRepo.DeleteByAppID(ctx, application.ID); err != nil {
		klog.Errorf("cleanup components for application %s failed: %v", application.ID, err)
	}
}

// UpdateVersion 更新应用版本，支持组件的更新、新增、删除操作
func (c *applicationsServiceImpl) UpdateVersion(ctx context.Context, appID string, req apisv1.UpdateVersionRequest) (*apisv1.UpdateVersionResponse, error) {
	if appID == "" {
		return nil, bcode.ErrApplicationNotExist
	}

	// 1. 获取应用信息
	app, err := c.AppRepo.FindByID(ctx, appID)
	if err != nil {
		if errors.Is(err, datastore.ErrRecordNotExist) {
			return nil, bcode.ErrApplicationNotExist
		}
		return nil, err
	}

	// 2. 获取应用组件
	components, err := c.ComponentRepo.FindByAppID(ctx, app.ID)
	if err != nil {
		return nil, err
	}

	// 3. 构建组件名称映射（用于快速查找）
	componentMap := make(map[string]*model.ApplicationComponent, len(components))
	for _, comp := range components {
		if comp != nil {
			componentMap[strings.ToLower(comp.Name)] = comp
		}
	}

	// 4. 解析更新策略
	strategy := config.ParseUpdateStrategy(req.Strategy)

	// 5. 保存旧版本号
	previousVersion := app.Version

	// 6. 使用请求中的版本号（必填字段）
	newVersion := req.Version

	// 7. 处理组件操作
	var (
		updatedComponents = make([]string, 0)
		addedComponents   = make([]string, 0)
		removedComponents = make([]string, 0)
	)

	for _, spec := range req.Components {
		action := config.ParseComponentAction(spec.Action)
		compName := strings.ToLower(strings.TrimSpace(spec.Name))

		switch action {
		case config.ComponentActionUpdate:
			// 更新现有组件
			comp, exists := componentMap[compName]
			if !exists {
				klog.Warningf("component %s not found for update, skipping", spec.Name)
				continue
			}
			if updated := c.updateComponent(ctx, comp, spec); updated {
				updatedComponents = append(updatedComponents, spec.Name)
			}

		case config.ComponentActionAdd:
			// 新增组件
			if _, exists := componentMap[compName]; exists {
				klog.Warningf("component %s already exists, skipping add", spec.Name)
				continue
			}
			if err := c.addComponent(ctx, app, spec); err != nil {
				klog.Errorf("add component %s failed: %v", spec.Name, err)
				return nil, bcode.ErrVersionUpdateFailed
			}
			addedComponents = append(addedComponents, spec.Name)

		case config.ComponentActionRemove:
			// 删除组件
			comp, exists := componentMap[compName]
			if !exists {
				klog.Warningf("component %s not found for removal, skipping", spec.Name)
				continue
			}
			if err := c.ComponentRepo.Delete(ctx, comp); err != nil {
				klog.Errorf("delete component %s failed: %v", spec.Name, err)
				return nil, bcode.ErrVersionUpdateFailed
			}
			removedComponents = append(removedComponents, spec.Name)
			delete(componentMap, compName)
		}
	}

	// 8. 更新应用版本号和描述
	app.Version = newVersion
	if req.Description != "" {
		app.Description = req.Description
	}
	if err := c.AppRepo.Update(ctx, app); err != nil {
		return nil, bcode.ErrVersionUpdateFailed
	}

	// 9. 更新工作流步骤（如果有组件增删）
	if len(addedComponents) > 0 || len(removedComponents) > 0 {
		if err := c.syncWorkflowSteps(ctx, app.ID, componentMap, addedComponents, removedComponents); err != nil {
			klog.Warningf("sync workflow steps failed: %v", err)
		}
	}

	// 10. 构造响应
	resp := &apisv1.UpdateVersionResponse{
		AppID:             app.ID,
		Version:           newVersion,
		PreviousVersion:   previousVersion,
		Strategy:          string(strategy),
		UpdatedComponents: updatedComponents,
		AddedComponents:   addedComponents,
		RemovedComponents: removedComponents,
	}

	// 11. 是否自动执行工作流
	autoExec := true
	if req.AutoExec != nil {
		autoExec = *req.AutoExec
	}

	hasChanges := len(updatedComponents) > 0 || len(addedComponents) > 0 || len(removedComponents) > 0
	if autoExec && hasChanges {
		// 查找默认工作流并执行
		workflows, err := c.WorkflowRepo.FindByAppID(ctx, app.ID)
		if err == nil && len(workflows) > 0 {
			// 使用第一个工作流执行
			taskID, err := c.execWorkflow(ctx, workflows[0])
			if err != nil {
				klog.Warningf("auto exec workflow failed: %v", err)
			} else {
				resp.TaskID = taskID
			}
		}
	}

	klog.Infof("AUDIT: update version appID=%s from=%s to=%s strategy=%s updated=%v added=%v removed=%v taskID=%s",
		app.ID, previousVersion, newVersion, strategy, updatedComponents, addedComponents, removedComponents, resp.TaskID)

	return resp, nil
}

// updateComponent 更新单个组件的配置
func (c *applicationsServiceImpl) updateComponent(ctx context.Context, comp *model.ApplicationComponent, spec apisv1.ComponentUpdateSpec) bool {
	changed := false

	// 更新镜像
	if spec.Image != "" && spec.Image != comp.Image {
		comp.Image = spec.Image
		changed = true
	}

	// 更新副本数
	if spec.Replicas != nil && *spec.Replicas != comp.Replicas {
		comp.Replicas = *spec.Replicas
		changed = true
	}

	// 更新环境变量
	if len(spec.Env) > 0 {
		if err := c.mergeComponentEnv(comp, spec.Env); err == nil {
			changed = true
		}
	}

	if changed {
		if err := c.ComponentRepo.Update(ctx, comp); err != nil {
			klog.Errorf("update component %s failed: %v", comp.Name, err)
			return false
		}
	}

	return changed
}

// addComponent 新增组件
func (c *applicationsServiceImpl) addComponent(ctx context.Context, app *model.Applications, spec apisv1.ComponentUpdateSpec) error {
	replicas := int32(1)
	if spec.Replicas != nil {
		replicas = *spec.Replicas
	}

	component := &model.ApplicationComponent{
		Name:          spec.Name,
		AppID:         app.ID,
		Namespace:     app.Namespace,
		Image:         spec.Image,
		Replicas:      replicas,
		ComponentType: spec.ComponentType,
	}

	// 设置 Properties
	if spec.Properties != nil {
		props, err := model.NewJSONStructByStruct(spec.Properties)
		if err != nil {
			return fmt.Errorf("marshal properties: %w", err)
		}
		component.Properties = props
	}

	// 设置 Traits
	if spec.Traits != nil {
		traits, err := model.NewJSONStructByStruct(spec.Traits)
		if err != nil {
			return fmt.Errorf("marshal traits: %w", err)
		}
		component.Traits = traits
	}

	return c.ComponentRepo.Create(ctx, component)
}

// mergeComponentEnv 合并组件环境变量
func (c *applicationsServiceImpl) mergeComponentEnv(comp *model.ApplicationComponent, envUpdates map[string]string) error {
	if comp.Properties == nil {
		return nil
	}

	var props apisv1.Properties
	if err := decodeJSONStruct(comp.Properties, &props); err != nil {
		return err
	}

	if props.Env == nil {
		props.Env = make(map[string]string)
	}
	for k, v := range envUpdates {
		props.Env[k] = v
	}

	newProps, err := model.NewJSONStructByStruct(props)
	if err != nil {
		return err
	}
	comp.Properties = newProps
	return nil
}

// syncWorkflowSteps 同步工作流步骤（组件增删后更新工作流）
func (c *applicationsServiceImpl) syncWorkflowSteps(ctx context.Context, appID string, componentMap map[string]*model.ApplicationComponent, added, removed []string) error {
	workflows, err := c.WorkflowRepo.FindByAppID(ctx, appID)
	if err != nil || len(workflows) == 0 {
		return err
	}

	// 只更新第一个工作流
	workflow := workflows[0]
	if workflow.Steps == nil {
		return nil
	}

	var steps model.WorkflowSteps
	if err := decodeJSONStruct(workflow.Steps, &steps); err != nil {
		return err
	}

	// 删除已移除组件的步骤
	removedSet := make(map[string]struct{}, len(removed))
	for _, name := range removed {
		removedSet[strings.ToLower(name)] = struct{}{}
	}

	filteredSteps := make([]*model.WorkflowStep, 0, len(steps.Steps))
	for _, step := range steps.Steps {
		if step == nil {
			continue
		}
		stepName := strings.ToLower(step.Name)
		if _, shouldRemove := removedSet[stepName]; !shouldRemove {
			filteredSteps = append(filteredSteps, step)
		}
	}

	// 添加新组件的步骤
	for _, name := range added {
		newStep := &model.WorkflowStep{
			Name:         name,
			WorkflowType: config.JobDeploy,
			Mode:         config.WorkflowModeStepByStep,
			Properties: []model.Policies{{
				Policies: []string{name},
			}},
		}
		filteredSteps = append(filteredSteps, newStep)
	}

	steps.Steps = filteredSteps

	// 更新工作流
	newSteps, err := model.NewJSONStructByStruct(&steps)
	if err != nil {
		return err
	}
	workflow.Steps = newSteps

	return c.WorkflowRepo.Update(ctx, workflow)
}

// execWorkflow 执行工作流
func (c *applicationsServiceImpl) execWorkflow(ctx context.Context, workflow *model.Workflow) (string, error) {
	if workflow == nil || workflow.Steps == nil {
		return "", fmt.Errorf("invalid workflow")
	}

	workflowTask := &model.WorkflowQueue{
		TaskID:              utils.RandStringByNumLowercase(24),
		AppID:               workflow.AppID,
		WorkflowID:          workflow.ID,
		ProjectID:           workflow.ProjectID,
		WorkflowName:        workflow.Name,
		WorkflowDisplayName: workflow.Alias,
		Type:                workflow.WorkflowType,
		Status:              config.StatusWaiting,
	}

	if err := c.WorkflowQueueRepo.Create(ctx, workflowTask); err != nil {
		klog.Errorf("create workflow queue failed: %v", err)
		return "", err
	}

	return workflowTask.TaskID, nil
}

// incrementVersion 递增版本号 (如 1.0.0 -> 1.0.1)
func incrementVersion(version string) string {
	if version == "" {
		return "1.0.1"
	}
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return version + ".1"
	}

	// 递增最后一位
	lastIdx := len(parts) - 1
	var lastNum int
	fmt.Sscanf(parts[lastIdx], "%d", &lastNum)
	parts[lastIdx] = fmt.Sprintf("%d", lastNum+1)

	return strings.Join(parts, ".")
}
