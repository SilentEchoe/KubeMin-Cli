package workflow

import (
	"context"
	"encoding/json"
	"sort"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/event/workflow/job"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

type StepExecution struct {
	Name string
	Mode config.WorkflowMode
	Jobs map[int][]*model.JobTask
}

func GenerateJobTasks(ctx context.Context, task *model.WorkflowQueue, ds datastore.DataStore, defaultJobTimeoutSeconds int64) []StepExecution {
	logger := klog.FromContext(ctx)
	workflow := model.Workflow{ID: task.WorkflowID}
	if err := ds.Get(ctx, &workflow); err != nil {
		logger.Error(err, "Failed to get workflow for generating job tasks", "workflowID", task.WorkflowID)
		return nil
	}

	stepsBytes, err := json.Marshal(workflow.Steps)
	if err != nil {
		logger.Error(err, "Failed to marshal workflow steps")
		return nil
	}

	var workflowSteps model.WorkflowSteps
	if err := json.Unmarshal(stepsBytes, &workflowSteps); err != nil {
		logger.Error(err, "Failed to unmarshal workflow steps")
		return nil
	}

	componentEntities, err := ds.List(ctx, &model.ApplicationComponent{AppID: task.AppID}, &datastore.ListOptions{})
	if err != nil {
		logger.Error(err, "Failed to list application components", "appID", task.AppID)
		return nil
	}
	componentMap := make(map[string]*model.ApplicationComponent)
	for _, entity := range componentEntities {
		if component, ok := entity.(*model.ApplicationComponent); ok {
			componentMap[component.Name] = component
		}
	}

	var executions []StepExecution
	totalJobs := 0

	for _, step := range workflowSteps.Steps {
		mode := step.Mode
		if mode == "" {
			mode = config.WorkflowModeStepByStep
		}
		if len(step.SubSteps) > 0 {
			if mode.IsParallel() {
				buckets := newJobBuckets()
				for _, sub := range step.SubSteps {
					subComponents := sub.ComponentNames()
					appendComponentGroup(ctx, buckets, subComponents, componentMap, task, defaultJobTimeoutSeconds)
				}
				if !bucketsEmpty(buckets) {
					totalJobs += countJobs(buckets)
					stepName := step.Name
					if stepName == "" {
						stepName = "parallel-group"
					}
					executions = append(executions, StepExecution{Name: stepName, Mode: mode, Jobs: buckets})
					logGeneratedJobs(logger, task.WorkflowName, stepName, mode, buckets)
				}
			} else {
				for _, sub := range step.SubSteps {
					buckets := newJobBuckets()
					subComponents := sub.ComponentNames()
					appendComponentGroup(ctx, buckets, subComponents, componentMap, task, defaultJobTimeoutSeconds)
					if bucketsEmpty(buckets) {
						continue
					}
					totalJobs += countJobs(buckets)
					displayName := sub.Name
					if displayName == "" && len(subComponents) == 1 {
						displayName = subComponents[0]
					}
					executions = append(executions, StepExecution{Name: displayName, Mode: config.WorkflowModeStepByStep, Jobs: buckets})
					logGeneratedJobs(logger, task.WorkflowName, displayName, config.WorkflowModeStepByStep, buckets)
				}
			}
			continue
		}

		componentNames := step.ComponentNames()
		if len(componentNames) == 0 {
			continue
		}
		if mode.IsParallel() && len(componentNames) > 1 {
			buckets := newJobBuckets()
			appendComponentGroup(ctx, buckets, componentNames, componentMap, task, defaultJobTimeoutSeconds)
			if !bucketsEmpty(buckets) {
				totalJobs += countJobs(buckets)
				stepName := step.Name
				if stepName == "" && len(componentNames) > 1 {
					stepName = "parallel-group"
				}
				executions = append(executions, StepExecution{Name: stepName, Mode: mode, Jobs: buckets})
				logGeneratedJobs(logger, task.WorkflowName, stepName, mode, buckets)
			}
			continue
		}
		for _, name := range componentNames {
			buckets := newJobBuckets()
			appendComponentGroup(ctx, buckets, []string{name}, componentMap, task, defaultJobTimeoutSeconds)
			if bucketsEmpty(buckets) {
				continue
			}
			totalJobs += countJobs(buckets)
			executions = append(executions, StepExecution{Name: name, Mode: config.WorkflowModeStepByStep, Jobs: buckets})
			logGeneratedJobs(logger, task.WorkflowName, name, config.WorkflowModeStepByStep, buckets)
		}
	}

	logger.Info("Generated total jobs for workflow", "totalJobs", totalJobs, "workflowName", task.WorkflowName)
	return executions
}

func NewJobTask(name, namespace, workflowID, projectID, appID, taskID string, timeoutSeconds int64) *model.JobTask {
	if timeoutSeconds <= 0 {
		timeoutSeconds = int64(config.DefaultJobTaskTimeout)
	}
	return &model.JobTask{
		Name:       name,
		Namespace:  namespace,
		WorkflowID: workflowID,
		ProjectID:  projectID,
		AppID:      appID,
		TaskID:     taskID,
		Status:     config.StatusQueued,
		Timeout:    timeoutSeconds,
	}
}

// setDeployTimeout forces deployment-related jobs to use the standard deploy timeout (20 minutes).
func setDeployTimeout(jobTask *model.JobTask) {
	if jobTask == nil {
		return
	}
	jobTask.Timeout = config.DeployTimeout
}

func ParseProperties(ctx context.Context, properties *model.JSONStruct) model.Properties {
	logger := klog.FromContext(ctx)
	cProperties, err := json.Marshal(properties)
	if err != nil {
		logger.Error(err, "Component.Properties deserialization failure")
		return model.Properties{}
	}

	var propertied model.Properties
	err = json.Unmarshal(cProperties, &propertied)
	if err != nil {
		logger.Error(err, "WorkflowSteps deserialization failure")
		return model.Properties{}
	}
	return propertied
}

func CreateObjectJobsFromResult(additionalObjects []client.Object, component *model.ApplicationComponent, task *model.WorkflowQueue, jobs []*model.JobTask, defaultJobTimeoutSeconds int64) ([]*model.JobTask, error) {
	if len(additionalObjects) == 0 {
		return jobs, nil
	}

	share := shareConfigForComponent(component)

	for _, obj := range additionalObjects {
		if pvc, ok := obj.(*corev1.PersistentVolumeClaim); ok {
			ns := pvc.Namespace
			if ns == "" {
				ns = component.Namespace
				pvc.Namespace = ns
			}
			applyShareLabelsToObject(pvc, share)
			pvcJob := NewJobTask(
				pvc.Name,
				ns,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
				task.TaskID,
				defaultJobTimeoutSeconds,
			)
			pvcJob.JobType = string(config.JobDeployPVC)
			pvcJob.JobInfo = pvc
			setDeployTimeout(pvcJob)
			markJobSkippedIfIgnored(share, pvcJob)

			jobs = append(jobs, pvcJob)
			klog.Infof("Created PVC job for component %s: %s", component.Name, pvc.Name)
		}
		if ingress, ok := obj.(*networkingv1.Ingress); ok {
			baseName := nameOrFallback(ingress.Name, component.Name)
			normalizedName := job.BuildIngressName(baseName, component.AppID)
			ingress.Name = normalizedName
			if ingress.Namespace == "" {
				ingress.Namespace = component.Namespace
			}
			applyShareLabelsToObject(ingress, share)
			ingressJob := NewJobTask(
				ingress.Name,
				ingress.Namespace,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
				task.TaskID,
				defaultJobTimeoutSeconds,
			)
			ingressJob.JobType = string(config.JobDeployIngress)
			ingressJob.JobInfo = ingress
			setDeployTimeout(ingressJob)
			markJobSkippedIfIgnored(share, ingressJob)
			jobs = append(jobs, ingressJob)
			klog.Infof("Created Ingress job for component %s: %s", component.Name, ingress.Name)
		}
		if sa, ok := obj.(*corev1.ServiceAccount); ok {
			ns := sa.Namespace
			if ns == "" {
				ns = component.Namespace
				sa.Namespace = ns
			}
			applyShareLabelsToObject(sa, share)
			jobTask := NewJobTask(
				sa.Name,
				ns,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
				task.TaskID,
				defaultJobTimeoutSeconds,
			)
			jobTask.JobType = string(config.JobDeployServiceAccount)
			jobTask.JobInfo = sa.DeepCopy()
			setDeployTimeout(jobTask)
			markJobSkippedIfIgnored(share, jobTask)
			jobs = append(jobs, jobTask)
			klog.Infof("Created ServiceAccount job for component %s: %s/%s", component.Name, ns, sa.Name)
		}
		if role, ok := obj.(*rbacv1.Role); ok {
			ns := role.Namespace
			if ns == "" {
				ns = component.Namespace
				role.Namespace = ns
			}
			applyShareLabelsToObject(role, share)
			jobTask := NewJobTask(
				role.Name,
				ns,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
				task.TaskID,
				defaultJobTimeoutSeconds,
			)
			jobTask.JobType = string(config.JobDeployRole)
			jobTask.JobInfo = role.DeepCopy()
			setDeployTimeout(jobTask)
			markJobSkippedIfIgnored(share, jobTask)
			jobs = append(jobs, jobTask)
			klog.Infof("Created Role job for component %s: %s/%s", component.Name, ns, role.Name)
		}
		if binding, ok := obj.(*rbacv1.RoleBinding); ok {
			ns := binding.Namespace
			if ns == "" {
				ns = component.Namespace
				binding.Namespace = ns
			}
			applyShareLabelsToObject(binding, share)
			jobTask := NewJobTask(
				binding.Name,
				ns,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
				task.TaskID,
				defaultJobTimeoutSeconds,
			)
			jobTask.JobType = string(config.JobDeployRoleBinding)
			jobTask.JobInfo = binding.DeepCopy()
			setDeployTimeout(jobTask)
			markJobSkippedIfIgnored(share, jobTask)
			jobs = append(jobs, jobTask)
			klog.Infof("Created RoleBinding job for component %s: %s/%s", component.Name, ns, binding.Name)
		}
		if clusterRole, ok := obj.(*rbacv1.ClusterRole); ok {
			applyShareLabelsToObject(clusterRole, share)
			jobTask := NewJobTask(
				clusterRole.Name,
				component.Namespace,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
				task.TaskID,
				defaultJobTimeoutSeconds,
			)
			jobTask.JobType = string(config.JobDeployClusterRole)
			jobTask.JobInfo = clusterRole.DeepCopy()
			setDeployTimeout(jobTask)
			markJobSkippedIfIgnored(share, jobTask)
			jobs = append(jobs, jobTask)
			klog.Infof("Created ClusterRole job for component %s: %s", component.Name, clusterRole.Name)
		}
		if clusterBinding, ok := obj.(*rbacv1.ClusterRoleBinding); ok {
			applyShareLabelsToObject(clusterBinding, share)
			jobTask := NewJobTask(
				clusterBinding.Name,
				component.Namespace,
				task.WorkflowID,
				task.ProjectID,
				task.AppID,
				task.TaskID,
				defaultJobTimeoutSeconds,
			)
			jobTask.JobType = string(config.JobDeployClusterRoleBinding)
			jobTask.JobInfo = clusterBinding.DeepCopy()
			setDeployTimeout(jobTask)
			markJobSkippedIfIgnored(share, jobTask)
			jobs = append(jobs, jobTask)
			klog.Infof("Created ClusterRoleBinding job for component %s: %s", component.Name, clusterBinding.Name)
		}
	}
	return jobs, nil
}

func appendComponentGroup(ctx context.Context, buckets map[int][]*model.JobTask, componentNames []string, componentMap map[string]*model.ApplicationComponent, task *model.WorkflowQueue, defaultJobTimeoutSeconds int64) {
	logger := klog.FromContext(ctx)
	for _, name := range componentNames {
		component, ok := componentMap[name]
		if !ok {
			logger.Info("Component referenced in workflow step not found", "componentName", name)
			continue
		}
		componentBuckets := buildJobsForComponent(ctx, component, task, defaultJobTimeoutSeconds)
		mergeJobBuckets(buckets, componentBuckets)
	}
}

func buildJobsForComponent(ctx context.Context, component *model.ApplicationComponent, task *model.WorkflowQueue, defaultJobTimeoutSeconds int64) map[int][]*model.JobTask {
	logger := klog.FromContext(ctx)
	buckets := newJobBuckets()
	if component == nil {
		return buckets
	}

	namespace := component.Namespace
	if namespace == "" {
		namespace = config.DefaultNamespace
		component.Namespace = namespace
	}

	properties := ParseProperties(ctx, component.Properties)
	share := shareConfigForComponent(component)

	switch component.ComponentType {
	case config.ServerJob:
		serviceJobs := job.GenerateWebService(component, &properties)
		queueServiceJobs(logger, buckets, component, task, namespace, config.JobDeploy, serviceJobs, defaultJobTimeoutSeconds, share)
	case config.StoreJob:
		storeJobs := job.GenerateStoreService(component)
		queueServiceJobs(logger, buckets, component, task, namespace, config.JobDeployStore, storeJobs, defaultJobTimeoutSeconds, share)

	case config.ConfJob:
		jobTask := NewJobTask(component.Name, namespace, task.WorkflowID, task.ProjectID, task.AppID, task.TaskID, defaultJobTimeoutSeconds)
		jobTask.JobType = string(config.JobDeployConfigMap)
		jobTask.JobInfo = job.GenerateConfigMap(component, &properties)
		applyShareLabelsToJobInfo(jobTask.JobInfo, share)
		setDeployTimeout(jobTask)
		markJobSkippedIfIgnored(share, jobTask)
		buckets[config.JobPriorityMaxHigh] = append(buckets[config.JobPriorityMaxHigh], jobTask)

	case config.SecretJob:
		jobTask := NewJobTask(component.Name, namespace, task.WorkflowID, task.ProjectID, task.AppID, task.TaskID, defaultJobTimeoutSeconds)
		jobTask.JobType = string(config.JobDeploySecret)
		jobTask.JobInfo = job.GenerateSecret(component, &properties)
		applyShareLabelsToJobInfo(jobTask.JobInfo, share)
		setDeployTimeout(jobTask)
		markJobSkippedIfIgnored(share, jobTask)
		buckets[config.JobPriorityMaxHigh] = append(buckets[config.JobPriorityMaxHigh], jobTask)
	}

	if len(properties.Ports) > 0 {
		svcJob := NewJobTask(component.Name, namespace, task.WorkflowID, task.ProjectID, task.AppID, task.TaskID, defaultJobTimeoutSeconds)
		svcJob.JobType = string(config.JobDeployService)
		svcJob.JobInfo = job.GenerateService(component, &properties)
		applyShareLabelsToJobInfo(svcJob.JobInfo, share)
		setDeployTimeout(svcJob)
		markJobSkippedIfIgnored(share, svcJob)
		buckets[config.JobPriorityNormal] = append(buckets[config.JobPriorityNormal], svcJob)
	}

	return buckets
}

func queueServiceJobs(
	logger klog.Logger,
	buckets map[int][]*model.JobTask,
	component *model.ApplicationComponent,
	task *model.WorkflowQueue,
	namespace string,
	jobType config.JobType,
	result *job.GenerateServiceResult,
	defaultJobTimeoutSeconds int64,
	share shareConfig,
) {
	if result == nil {
		return
	}

	appendJob := func(priority int, jobTask *model.JobTask) {
		if jobTask == nil {
			return
		}
		buckets[priority] = append(buckets[priority], jobTask)
	}

	// Traits may emit extra Kubernetes objects (PVC, Ingress, etc.). Schedule them
	// ahead of the base workload so dependencies are ready before the deployment runs.
	if len(result.AdditionalObjects) > 0 {
		jobs, err := CreateObjectJobsFromResult(result.AdditionalObjects, component, task, nil, defaultJobTimeoutSeconds)
		if err != nil {
			logger.Error(err, "Failed to create additional resource jobs", "componentName", component.Name)
		} else {
			for _, jt := range jobs {
				appendJob(config.JobPriorityHigh, jt)
			}
		}
	}

	jobTask := NewJobTask(component.Name, namespace, task.WorkflowID, task.ProjectID, task.AppID, task.TaskID, defaultJobTimeoutSeconds)
	jobTask.JobType = string(jobType)
	jobTask.JobInfo = result.Service
	applyShareLabelsToJobInfo(jobTask.JobInfo, share)
	setDeployTimeout(jobTask)
	markJobSkippedIfIgnored(share, jobTask)
	appendJob(config.JobPriorityNormal, jobTask)
}

func markJobSkippedIfIgnored(share shareConfig, jobTask *model.JobTask) {
	if jobTask == nil {
		return
	}
	if share.ignore() {
		jobTask.Status = config.StatusSkipped
		jobTask.Error = ""
	}
}

func newJobBuckets() map[int][]*model.JobTask {
	return map[int][]*model.JobTask{
		config.JobPriorityMaxHigh: {},
		config.JobPriorityHigh:    {},
		config.JobPriorityNormal:  {},
		config.JobPriorityLow:     {},
	}
}

func mergeJobBuckets(dst, src map[int][]*model.JobTask) {
	for priority, jobs := range src {
		if len(jobs) == 0 {
			continue
		}
		dst[priority] = append(dst[priority], jobs...)
	}
}

func bucketsEmpty(buckets map[int][]*model.JobTask) bool {
	for _, jobs := range buckets {
		if len(jobs) > 0 {
			return false
		}
	}
	return true
}

func countJobs(buckets map[int][]*model.JobTask) int {
	count := 0
	for _, jobs := range buckets {
		count += len(jobs)
	}
	return count
}

func logGeneratedJobs(logger klog.Logger, workflowName, stepName string, mode config.WorkflowMode, buckets map[int][]*model.JobTask) {
	for priority, jobs := range buckets {
		if len(jobs) == 0 {
			continue
		}
		logger.Info("Generated jobs for workflow step", "workflowName", workflowName, "step", stepName, "mode", mode, "priority", priority, "jobCount", len(jobs))
		for _, j := range jobs {
			logger.Info("Generated job details", "workflowName", workflowName, "step", stepName, "jobName", j.Name, "jobType", j.JobType, "priority", priority)
		}
	}
}

func determineStepConcurrency(mode config.WorkflowMode, jobCount, sequentialLimit int) int {
	if jobCount <= 0 {
		return 0
	}
	if mode.IsParallel() {
		return jobCount
	}
	if sequentialLimit < 1 {
		sequentialLimit = 1
	}
	if jobCount < sequentialLimit {
		return jobCount
	}
	return sequentialLimit
}

func sortedPriorities(jobs map[int][]*model.JobTask) []int {
	var priorities []int
	for priority := range jobs {
		priorities = append(priorities, priority)
	}
	sort.Ints(priorities)
	return priorities
}

func nameOrFallback(name, fallback string) string {
	if name != "" {
		return name
	}
	return fallback
}
