package job

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
)

// DeployServiceAccountJobCtl creates or updates a ServiceAccount resource.
type DeployServiceAccountJobCtl struct {
	namespace string
	job       *model.JobTask
	client    kubernetes.Interface
	store     datastore.DataStore
	ack       func()
}

func NewDeployServiceAccountJobCtl(job *model.JobTask, client kubernetes.Interface, store datastore.DataStore, ack func()) *DeployServiceAccountJobCtl {
	if job == nil {
		klog.Errorf("DeployServiceAccountJobCtl: job is nil")
		return nil
	}
	return &DeployServiceAccountJobCtl{
		namespace: job.Namespace,
		job:       job,
		client:    client,
		store:     store,
		ack:       ack,
	}
}

func (c *DeployServiceAccountJobCtl) Clean(ctx context.Context) {
	if c.client == nil {
		return
	}
	refs := resourcesForCleanup(ctx, config.ResourceServiceAccount)
	if len(refs) == 0 {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), config.DelTimeOut)
	defer cancel()
	for _, ref := range refs {
		if !ref.Created {
			continue
		}
		ns := ref.Namespace
		if ns == "" {
			ns = c.namespace
		}
		if err := c.client.CoreV1().ServiceAccounts(ns).Delete(cleanupCtx, ref.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			klog.Errorf("failed to delete serviceAccount %s/%s during cleanup: %v", ns, ref.Name, err)
		}
	}
}

func (c *DeployServiceAccountJobCtl) SaveInfo(ctx context.Context) error {
	jobInfo := model.JobInfo{
		Type:        c.job.JobType,
		WorkflowID:  c.job.WorkflowID,
		ProductID:   c.job.ProjectID,
		AppID:       c.job.AppID,
		TaskID:      c.job.TaskID,
		Status:      string(c.job.Status),
		StartTime:   c.job.StartTime,
		EndTime:     c.job.EndTime,
		Error:       c.job.Error,
		ServiceName: c.job.Name,
	}
	return c.store.Add(ctx, &jobInfo)
}

func (c *DeployServiceAccountJobCtl) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	c.job.Status = config.StatusRunning
	c.job.Error = ""
	c.ack()

	if err := c.run(ctx); err != nil {
		logger.Error(err, "DeployServiceAccountJob run error")
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return err
	}
	if c.job.Status == config.StatusSkipped {
		c.job.Error = ""
		return nil
	}
	c.job.Status = config.StatusCompleted
	c.job.Error = ""
	return nil
}

func (c *DeployServiceAccountJobCtl) run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	sa, ok := c.job.JobInfo.(*corev1.ServiceAccount)
	if !ok {
		return fmt.Errorf("job info is not *corev1.ServiceAccount")
	}

	if sa.Namespace == "" {
		sa.Namespace = c.job.Namespace
	}
	ensureManagedLabel(sa, c.job.AppID)

	cli := c.client.CoreV1().ServiceAccounts(sa.Namespace)
	shareName, shareStrategy := shareInfoFromLabels(sa.Labels)
	if shareStrategy == config.ShareStrategyIgnore {
		logger.Info("serviceAccount marked as shared ignore; skipping", "namespace", sa.Namespace, "name", sa.Name)
		c.job.Status = config.StatusSkipped
		c.job.Error = ""
		c.ack()
		return nil
	}
	if shareStrategy == config.ShareStrategyDefault {
		exists, err := hasSharedResources(ctx, shareName, func(ctx context.Context, opts metav1.ListOptions) (int, error) {
			list, err := cli.List(ctx, opts)
			if err != nil {
				return 0, err
			}
			return len(list.Items), nil
		})
		if err != nil {
			return fmt.Errorf("list shared serviceAccounts failed: %w", err)
		}
		if exists {
			logger.Info("serviceAccount already exists and is shared; skipping", "namespace", sa.Namespace, "name", sa.Name)
			c.job.Status = config.StatusSkipped
			c.job.Error = ""
			c.ack()
			return nil
		}
	}
	existing, err := cli.Get(ctx, sa.Name, metav1.GetOptions{})
	switch {
	case err == nil:
		if !isManagedResource(existing, c.job.AppID) {
			logger.Info("serviceAccount exists but is not managed; skipping update", "namespace", sa.Namespace, "name", sa.Name)
			markResourceObserved(ctx, config.ResourceServiceAccount, sa.Namespace, sa.Name)
			return nil
		}
		if shouldUpdateServiceAccount(existing, sa) {
			sa.ResourceVersion = existing.ResourceVersion
			if _, err := cli.Update(ctx, sa, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update serviceAccount %q failed: %w", sa.Name, err)
			}
			logger.Info("serviceAccount updated", "namespace", sa.Namespace, "name", sa.Name)
		} else {
			logger.Info("serviceAccount is up-to-date, skipping update", "namespace", sa.Namespace, "name", sa.Name)
		}
		markResourceObserved(ctx, config.ResourceServiceAccount, sa.Namespace, sa.Name)
	case k8serrors.IsNotFound(err):
		if _, err := cli.Create(ctx, sa, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create serviceAccount %q failed: %w", sa.Name, err)
		}
		logger.Info("serviceAccount created", "namespace", sa.Namespace, "name", sa.Name)
		MarkResourceCreated(ctx, config.ResourceServiceAccount, sa.Namespace, sa.Name)
	default:
		return fmt.Errorf("get serviceAccount %q failed: %w", sa.Name, err)
	}
	return nil
}

func (c *DeployServiceAccountJobCtl) wait(ctx context.Context) {}

// DeployRoleJobCtl reconciles namespace-scoped Role objects.
type DeployRoleJobCtl struct {
	job    *model.JobTask
	client kubernetes.Interface
	store  datastore.DataStore
	ack    func()
}

func NewDeployRoleJobCtl(job *model.JobTask, client kubernetes.Interface, store datastore.DataStore, ack func()) *DeployRoleJobCtl {
	if job == nil {
		klog.Errorf("DeployRoleJobCtl: job is nil")
		return nil
	}
	return &DeployRoleJobCtl{
		job:    job,
		client: client,
		store:  store,
		ack:    ack,
	}
}

func (c *DeployRoleJobCtl) Clean(ctx context.Context) {
	if c.client == nil {
		return
	}
	refs := resourcesForCleanup(ctx, config.ResourceRole)
	if len(refs) == 0 {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), config.DelTimeOut)
	defer cancel()
	for _, ref := range refs {
		if !ref.Created {
			continue
		}
		ns := ref.Namespace
		if ns == "" {
			ns = c.job.Namespace
		}
		if err := c.client.RbacV1().Roles(ns).Delete(cleanupCtx, ref.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			klog.Errorf("failed to delete role %s/%s during cleanup: %v", ns, ref.Name, err)
		}
	}
}

func (c *DeployRoleJobCtl) SaveInfo(ctx context.Context) error {
	jobInfo := model.JobInfo{
		Type:        c.job.JobType,
		WorkflowID:  c.job.WorkflowID,
		ProductID:   c.job.ProjectID,
		AppID:       c.job.AppID,
		TaskID:      c.job.TaskID,
		Status:      string(c.job.Status),
		StartTime:   c.job.StartTime,
		EndTime:     c.job.EndTime,
		Error:       c.job.Error,
		ServiceName: c.job.Name,
	}
	return c.store.Add(ctx, &jobInfo)
}

func (c *DeployRoleJobCtl) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	c.job.Status = config.StatusRunning
	c.job.Error = ""
	c.ack()

	if err := c.run(ctx); err != nil {
		logger.Error(err, "DeployRoleJob run error")
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return err
	}
	if c.job.Status == config.StatusSkipped {
		c.job.Error = ""
		return nil
	}
	c.job.Status = config.StatusCompleted
	c.job.Error = ""
	return nil
}

func (c *DeployRoleJobCtl) run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	role, ok := c.job.JobInfo.(*rbacv1.Role)
	if !ok {
		return fmt.Errorf("job info is not *rbacv1.Role")
	}
	if role.Namespace == "" {
		role.Namespace = c.job.Namespace
	}
	ensureManagedLabel(role, c.job.AppID)

	cli := c.client.RbacV1().Roles(role.Namespace)
	shareName, shareStrategy := shareInfoFromLabels(role.Labels)
	if shareStrategy == config.ShareStrategyIgnore {
		logger.Info("role marked as shared ignore; skipping", "namespace", role.Namespace, "name", role.Name)
		c.job.Status = config.StatusSkipped
		c.job.Error = ""
		c.ack()
		return nil
	}
	if shareStrategy == config.ShareStrategyDefault {
		exists, err := hasSharedResources(ctx, shareName, func(ctx context.Context, opts metav1.ListOptions) (int, error) {
			list, err := cli.List(ctx, opts)
			if err != nil {
				return 0, err
			}
			return len(list.Items), nil
		})
		if err != nil {
			return fmt.Errorf("list shared roles failed: %w", err)
		}
		if exists {
			logger.Info("role already exists and is shared; skipping", "namespace", role.Namespace, "name", role.Name)
			c.job.Status = config.StatusSkipped
			c.job.Error = ""
			c.ack()
			return nil
		}
	}
	existing, err := cli.Get(ctx, role.Name, metav1.GetOptions{})
	switch {
	case err == nil:
		if !isManagedResource(existing, c.job.AppID) {
			logger.Info("role exists but is not managed; skipping update", "namespace", role.Namespace, "name", role.Name)
			markResourceObserved(ctx, config.ResourceRole, role.Namespace, role.Name)
			return nil
		}
		if shouldUpdateRole(existing, role) {
			role.ResourceVersion = existing.ResourceVersion
			if _, err := cli.Update(ctx, role, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update role %q failed: %w", role.Name, err)
			}
			logger.Info("role updated", "namespace", role.Namespace, "name", role.Name)
		} else {
			logger.Info("role is up-to-date, skipping update", "namespace", role.Namespace, "name", role.Name)
		}
		markResourceObserved(ctx, config.ResourceRole, role.Namespace, role.Name)
	case k8serrors.IsNotFound(err):
		if _, err := cli.Create(ctx, role, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create role %q failed: %w", role.Name, err)
		}
		logger.Info("role created", "namespace", role.Namespace, "name", role.Name)
		MarkResourceCreated(ctx, config.ResourceRole, role.Namespace, role.Name)
	default:
		return fmt.Errorf("get role %q failed: %w", role.Name, err)
	}
	return nil
}

// DeployRoleBindingJobCtl reconciles RoleBinding objects.
type DeployRoleBindingJobCtl struct {
	job    *model.JobTask
	client kubernetes.Interface
	store  datastore.DataStore
	ack    func()
}

func NewDeployRoleBindingJobCtl(job *model.JobTask, client kubernetes.Interface, store datastore.DataStore, ack func()) *DeployRoleBindingJobCtl {
	if job == nil {
		klog.Errorf("DeployRoleBindingJobCtl: job is nil")
		return nil
	}
	return &DeployRoleBindingJobCtl{
		job:    job,
		client: client,
		store:  store,
		ack:    ack,
	}
}

func (c *DeployRoleBindingJobCtl) Clean(ctx context.Context) {
	if c.client == nil {
		return
	}
	refs := resourcesForCleanup(ctx, config.ResourceRoleBinding)
	if len(refs) == 0 {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), config.DelTimeOut)
	defer cancel()
	for _, ref := range refs {
		if !ref.Created {
			continue
		}
		ns := ref.Namespace
		if ns == "" {
			ns = c.job.Namespace
		}
		if err := c.client.RbacV1().RoleBindings(ns).Delete(cleanupCtx, ref.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			klog.Errorf("failed to delete roleBinding %s/%s during cleanup: %v", ns, ref.Name, err)
		}
	}
}

func (c *DeployRoleBindingJobCtl) SaveInfo(ctx context.Context) error {
	jobInfo := model.JobInfo{
		Type:        c.job.JobType,
		WorkflowID:  c.job.WorkflowID,
		ProductID:   c.job.ProjectID,
		AppID:       c.job.AppID,
		TaskID:      c.job.TaskID,
		Status:      string(c.job.Status),
		StartTime:   c.job.StartTime,
		EndTime:     c.job.EndTime,
		Error:       c.job.Error,
		ServiceName: c.job.Name,
	}
	return c.store.Add(ctx, &jobInfo)
}

func (c *DeployRoleBindingJobCtl) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	c.job.Status = config.StatusRunning
	c.job.Error = ""
	c.ack()

	if err := c.run(ctx); err != nil {
		logger.Error(err, "DeployRoleBindingJob run error")
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return err
	}
	if c.job.Status == config.StatusSkipped {
		c.job.Error = ""
		return nil
	}
	c.job.Status = config.StatusCompleted
	c.job.Error = ""
	return nil
}

func (c *DeployRoleBindingJobCtl) run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	binding, ok := c.job.JobInfo.(*rbacv1.RoleBinding)
	if !ok {
		return fmt.Errorf("job info is not *rbacv1.RoleBinding")
	}
	if binding.Namespace == "" {
		binding.Namespace = c.job.Namespace
	}
	ensureManagedLabel(binding, c.job.AppID)

	cli := c.client.RbacV1().RoleBindings(binding.Namespace)
	shareName, shareStrategy := shareInfoFromLabels(binding.Labels)
	if shareStrategy == config.ShareStrategyIgnore {
		logger.Info("roleBinding marked as shared ignore; skipping", "namespace", binding.Namespace, "name", binding.Name)
		c.job.Status = config.StatusSkipped
		c.job.Error = ""
		c.ack()
		return nil
	}
	if shareStrategy == config.ShareStrategyDefault {
		exists, err := hasSharedResources(ctx, shareName, func(ctx context.Context, opts metav1.ListOptions) (int, error) {
			list, err := cli.List(ctx, opts)
			if err != nil {
				return 0, err
			}
			return len(list.Items), nil
		})
		if err != nil {
			return fmt.Errorf("list shared roleBindings failed: %w", err)
		}
		if exists {
			logger.Info("roleBinding already exists and is shared; skipping", "namespace", binding.Namespace, "name", binding.Name)
			c.job.Status = config.StatusSkipped
			c.job.Error = ""
			c.ack()
			return nil
		}
	}
	existing, err := cli.Get(ctx, binding.Name, metav1.GetOptions{})
	switch {
	case err == nil:
		if !isManagedResource(existing, c.job.AppID) {
			logger.Info("roleBinding exists but is not managed; skipping update", "namespace", binding.Namespace, "name", binding.Name)
			markResourceObserved(ctx, config.ResourceRoleBinding, binding.Namespace, binding.Name)
			return nil
		}
		if shouldUpdateRoleBinding(existing, binding) {
			binding.ResourceVersion = existing.ResourceVersion
			if _, err := cli.Update(ctx, binding, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update roleBinding %q failed: %w", binding.Name, err)
			}
			logger.Info("roleBinding updated", "namespace", binding.Namespace, "name", binding.Name)
		} else {
			logger.Info("roleBinding is up-to-date, skipping update", "namespace", binding.Namespace, "name", binding.Name)
		}
		markResourceObserved(ctx, config.ResourceRoleBinding, binding.Namespace, binding.Name)
	case k8serrors.IsNotFound(err):
		if _, err := cli.Create(ctx, binding, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create roleBinding %q failed: %w", binding.Name, err)
		}
		logger.Info("roleBinding created", "namespace", binding.Namespace, "name", binding.Name)
		MarkResourceCreated(ctx, config.ResourceRoleBinding, binding.Namespace, binding.Name)
	default:
		return fmt.Errorf("get roleBinding %q failed: %w", binding.Name, err)
	}
	return nil
}

// DeployClusterRoleJobCtl reconciles ClusterRole objects.
type DeployClusterRoleJobCtl struct {
	job    *model.JobTask
	client kubernetes.Interface
	store  datastore.DataStore
	ack    func()
}

func NewDeployClusterRoleJobCtl(job *model.JobTask, client kubernetes.Interface, store datastore.DataStore, ack func()) *DeployClusterRoleJobCtl {
	if job == nil {
		klog.Errorf("DeployClusterRoleJobCtl: job is nil")
		return nil
	}
	return &DeployClusterRoleJobCtl{
		job:    job,
		client: client,
		store:  store,
		ack:    ack,
	}
}

func (c *DeployClusterRoleJobCtl) Clean(ctx context.Context) {
	if c.client == nil {
		return
	}
	refs := resourcesForCleanup(ctx, config.ResourceClusterRole)
	if len(refs) == 0 {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), config.DelTimeOut)
	defer cancel()
	for _, ref := range refs {
		if !ref.Created {
			continue
		}
		if err := c.client.RbacV1().ClusterRoles().Delete(cleanupCtx, ref.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			klog.Errorf("failed to delete clusterRole %s during cleanup: %v", ref.Name, err)
		}
	}
}

func (c *DeployClusterRoleJobCtl) SaveInfo(ctx context.Context) error {
	jobInfo := model.JobInfo{
		Type:        c.job.JobType,
		WorkflowID:  c.job.WorkflowID,
		ProductID:   c.job.ProjectID,
		AppID:       c.job.AppID,
		TaskID:      c.job.TaskID,
		Status:      string(c.job.Status),
		StartTime:   c.job.StartTime,
		EndTime:     c.job.EndTime,
		Error:       c.job.Error,
		ServiceName: c.job.Name,
	}
	return c.store.Add(ctx, &jobInfo)
}

func (c *DeployClusterRoleJobCtl) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	c.job.Status = config.StatusRunning
	c.job.Error = ""
	c.ack()

	if err := c.run(ctx); err != nil {
		logger.Error(err, "DeployClusterRoleJob run error")
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return err
	}
	if c.job.Status == config.StatusSkipped {
		c.job.Error = ""
		return nil
	}
	c.job.Status = config.StatusCompleted
	c.job.Error = ""
	return nil
}

func (c *DeployClusterRoleJobCtl) run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	role, ok := c.job.JobInfo.(*rbacv1.ClusterRole)
	if !ok {
		return fmt.Errorf("job info is not *rbacv1.ClusterRole")
	}
	ensureManagedLabel(role, c.job.AppID)

	cli := c.client.RbacV1().ClusterRoles()
	shareName, shareStrategy := shareInfoFromLabels(role.Labels)
	if shareStrategy == config.ShareStrategyIgnore {
		logger.Info("clusterRole marked as shared ignore; skipping", "name", role.Name)
		c.job.Status = config.StatusSkipped
		c.job.Error = ""
		c.ack()
		return nil
	}
	if shareStrategy == config.ShareStrategyDefault {
		exists, err := hasSharedResources(ctx, shareName, func(ctx context.Context, opts metav1.ListOptions) (int, error) {
			list, err := cli.List(ctx, opts)
			if err != nil {
				return 0, err
			}
			return len(list.Items), nil
		})
		if err != nil {
			return fmt.Errorf("list shared clusterRoles failed: %w", err)
		}
		if exists {
			logger.Info("clusterRole already exists and is shared; skipping", "name", role.Name)
			c.job.Status = config.StatusSkipped
			c.job.Error = ""
			c.ack()
			return nil
		}
	}
	existing, err := cli.Get(ctx, role.Name, metav1.GetOptions{})
	switch {
	case err == nil:
		if !isManagedResource(existing, c.job.AppID) {
			logger.Info("clusterRole exists but is not managed; skipping update", "name", role.Name)
			markResourceObserved(ctx, config.ResourceClusterRole, "", role.Name)
			return nil
		}
		if shouldUpdateClusterRole(existing, role) {
			role.ResourceVersion = existing.ResourceVersion
			if _, err := cli.Update(ctx, role, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update clusterRole %q failed: %w", role.Name, err)
			}
			logger.Info("clusterRole updated", "name", role.Name)
		} else {
			logger.Info("clusterRole is up-to-date, skipping update", "name", role.Name)
		}
		markResourceObserved(ctx, config.ResourceClusterRole, "", role.Name)
	case k8serrors.IsNotFound(err):
		if _, err := cli.Create(ctx, role, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create clusterRole %q failed: %w", role.Name, err)
		}
		logger.Info("clusterRole created", "name", role.Name)
		MarkResourceCreated(ctx, config.ResourceClusterRole, "", role.Name)
	default:
		return fmt.Errorf("get clusterRole %q failed: %w", role.Name, err)
	}
	return nil
}

// DeployClusterRoleBindingJobCtl reconciles ClusterRoleBinding objects.
type DeployClusterRoleBindingJobCtl struct {
	job    *model.JobTask
	client kubernetes.Interface
	store  datastore.DataStore
	ack    func()
}

func NewDeployClusterRoleBindingJobCtl(job *model.JobTask, client kubernetes.Interface, store datastore.DataStore, ack func()) *DeployClusterRoleBindingJobCtl {
	if job == nil {
		klog.Errorf("DeployClusterRoleBindingJobCtl: job is nil")
		return nil
	}
	return &DeployClusterRoleBindingJobCtl{
		job:    job,
		client: client,
		store:  store,
		ack:    ack,
	}
}

func (c *DeployClusterRoleBindingJobCtl) Clean(ctx context.Context) {
	if c.client == nil {
		return
	}
	refs := resourcesForCleanup(ctx, config.ResourceClusterRoleBinding)
	if len(refs) == 0 {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), config.DelTimeOut)
	defer cancel()
	for _, ref := range refs {
		if !ref.Created {
			continue
		}
		if err := c.client.RbacV1().ClusterRoleBindings().Delete(cleanupCtx, ref.Name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			klog.Errorf("failed to delete clusterRoleBinding %s during cleanup: %v", ref.Name, err)
		}
	}
}

func (c *DeployClusterRoleBindingJobCtl) SaveInfo(ctx context.Context) error {
	jobInfo := model.JobInfo{
		Type:        c.job.JobType,
		WorkflowID:  c.job.WorkflowID,
		ProductID:   c.job.ProjectID,
		AppID:       c.job.AppID,
		TaskID:      c.job.TaskID,
		Status:      string(c.job.Status),
		StartTime:   c.job.StartTime,
		EndTime:     c.job.EndTime,
		Error:       c.job.Error,
		ServiceName: c.job.Name,
	}
	return c.store.Add(ctx, &jobInfo)
}

func (c *DeployClusterRoleBindingJobCtl) Run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	c.job.Status = config.StatusRunning
	c.job.Error = ""
	c.ack()

	if err := c.run(ctx); err != nil {
		logger.Error(err, "DeployClusterRoleBindingJob run error")
		c.job.Status = config.StatusFailed
		c.job.Error = err.Error()
		return err
	}
	if c.job.Status == config.StatusSkipped {
		c.job.Error = ""
		return nil
	}
	c.job.Status = config.StatusCompleted
	c.job.Error = ""
	return nil
}

func (c *DeployClusterRoleBindingJobCtl) run(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	if c.client == nil {
		return fmt.Errorf("client is nil")
	}

	binding, ok := c.job.JobInfo.(*rbacv1.ClusterRoleBinding)
	if !ok {
		return fmt.Errorf("job info is not *rbacv1.ClusterRoleBinding")
	}
	ensureManagedLabel(binding, c.job.AppID)

	cli := c.client.RbacV1().ClusterRoleBindings()
	shareName, shareStrategy := shareInfoFromLabels(binding.Labels)
	if shareStrategy == config.ShareStrategyIgnore {
		logger.Info("clusterRoleBinding marked as shared ignore; skipping", "name", binding.Name)
		c.job.Status = config.StatusSkipped
		c.job.Error = ""
		c.ack()
		return nil
	}
	if shareStrategy == config.ShareStrategyDefault {
		exists, err := hasSharedResources(ctx, shareName, func(ctx context.Context, opts metav1.ListOptions) (int, error) {
			list, err := cli.List(ctx, opts)
			if err != nil {
				return 0, err
			}
			return len(list.Items), nil
		})
		if err != nil {
			return fmt.Errorf("list shared clusterRoleBindings failed: %w", err)
		}
		if exists {
			logger.Info("clusterRoleBinding already exists and is shared; skipping", "name", binding.Name)
			c.job.Status = config.StatusSkipped
			c.job.Error = ""
			c.ack()
			return nil
		}
	}
	existing, err := cli.Get(ctx, binding.Name, metav1.GetOptions{})
	switch {
	case err == nil:
		if !isManagedResource(existing, c.job.AppID) {
			logger.Info("clusterRoleBinding exists but is not managed; skipping update", "name", binding.Name)
			markResourceObserved(ctx, config.ResourceClusterRoleBinding, "", binding.Name)
			return nil
		}
		if shouldUpdateClusterRoleBinding(existing, binding) {
			binding.ResourceVersion = existing.ResourceVersion
			if _, err := cli.Update(ctx, binding, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update clusterRoleBinding %q failed: %w", binding.Name, err)
			}
			logger.Info("clusterRoleBinding updated", "name", binding.Name)
		} else {
			logger.Info("clusterRoleBinding is up-to-date, skipping update", "name", binding.Name)
		}
		markResourceObserved(ctx, config.ResourceClusterRoleBinding, "", binding.Name)
	case k8serrors.IsNotFound(err):
		if _, err := cli.Create(ctx, binding, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create clusterRoleBinding %q failed: %w", binding.Name, err)
		}
		logger.Info("clusterRoleBinding created", "name", binding.Name)
		MarkResourceCreated(ctx, config.ResourceClusterRoleBinding, "", binding.Name)
	default:
		return fmt.Errorf("get clusterRoleBinding %q failed: %w", binding.Name, err)
	}
	return nil
}

func shouldUpdateServiceAccount(existing, desired *corev1.ServiceAccount) bool {
	if !apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels) {
		return true
	}
	if !apiequality.Semantic.DeepEqual(existing.Annotations, desired.Annotations) {
		return true
	}
	return !apiequality.Semantic.DeepEqual(existing.AutomountServiceAccountToken, desired.AutomountServiceAccountToken)
}

func shouldUpdateRole(existing, desired *rbacv1.Role) bool {
	if !apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels) {
		return true
	}
	if !apiequality.Semantic.DeepEqual(existing.Annotations, desired.Annotations) {
		return true
	}
	return !apiequality.Semantic.DeepEqual(existing.Rules, desired.Rules)
}

func shouldUpdateRoleBinding(existing, desired *rbacv1.RoleBinding) bool {
	if !apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels) {
		return true
	}
	if !apiequality.Semantic.DeepEqual(existing.Annotations, desired.Annotations) {
		return true
	}
	if !apiequality.Semantic.DeepEqual(existing.Subjects, desired.Subjects) {
		return true
	}
	return !apiequality.Semantic.DeepEqual(existing.RoleRef, desired.RoleRef)
}

func shouldUpdateClusterRole(existing, desired *rbacv1.ClusterRole) bool {
	if !apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels) {
		return true
	}
	if !apiequality.Semantic.DeepEqual(existing.Annotations, desired.Annotations) {
		return true
	}
	return !apiequality.Semantic.DeepEqual(existing.Rules, desired.Rules)
}

func shouldUpdateClusterRoleBinding(existing, desired *rbacv1.ClusterRoleBinding) bool {
	if !apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels) {
		return true
	}
	if !apiequality.Semantic.DeepEqual(existing.Annotations, desired.Annotations) {
		return true
	}
	if !apiequality.Semantic.DeepEqual(existing.Subjects, desired.Subjects) {
		return true
	}
	return !apiequality.Semantic.DeepEqual(existing.RoleRef, desired.RoleRef)
}

func ensureManagedLabel(obj metav1.Object, appID string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 1)
	}
	labels[config.LabelCli] = appID
	obj.SetLabels(labels)
}

func isManagedResource(obj metav1.Object, expected string) bool {
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	return labels[config.LabelCli] == expected
}
