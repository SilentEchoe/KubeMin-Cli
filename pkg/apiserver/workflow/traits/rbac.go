package traits

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"KubeMin-Cli/pkg/apiserver/config"
	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
	"KubeMin-Cli/pkg/apiserver/utils"
)

// RBACProcessor materializes RBAC resources (ServiceAccount, Role/ClusterRole, RoleBinding)
// based on declarative trait specifications.
type RBACProcessor struct{}

func (p *RBACProcessor) Name() string { return "rbac" }

func (p *RBACProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	specs, ok := ctx.TraitData.([]spec.RBACPolicySpec)
	if !ok {
		return nil, fmt.Errorf("rbac trait expects []spec.RBACPolicySpec, got %T", ctx.TraitData)
	}
	if len(specs) == 0 {
		return nil, nil
	}

	component := ctx.Component
	if component == nil {
		return nil, fmt.Errorf("rbac processor requires component context")
	}

	result := &TraitResult{}
	for idx, policy := range specs {
		namespace := firstNonEmpty(policy.Namespace, component.Namespace, config.DefaultNamespace)

		saName := policy.ServiceAccount
		if saName == "" {
			base := utils.NormalizeLowerStrip(component.Name)
			if base == "" {
				base = "service"
			}
			if idx == 0 {
				saName = fmt.Sprintf("%s-sa", base)
			} else {
				saName = fmt.Sprintf("%s-%d-sa", base, idx+1)
			}
		} else {
			saName = utils.NormalizeLowerStrip(saName)
		}

		roleName := policy.RoleName
		if roleName == "" {
			roleName = fmt.Sprintf("%s-role", saName)
		} else {
			roleName = utils.NormalizeLowerStrip(roleName)
		}

		bindingName := policy.BindingName
		if bindingName == "" {
			bindingName = fmt.Sprintf("%s-binding", saName)
		} else {
			bindingName = utils.NormalizeLowerStrip(bindingName)
		}

		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:        saName,
				Namespace:   namespace,
				Labels:      utils.CopyStringMap(policy.ServiceAccountLabels),
				Annotations: utils.CopyStringMap(policy.ServiceAccountAnnotations),
			},
		}
		if policy.ServiceAccountAutomountSAT != nil {
			sa.AutomountServiceAccountToken = policy.ServiceAccountAutomountSAT
		}
		result.AdditionalObjects = append(result.AdditionalObjects, sa)

		if result.ServiceAccountName == "" {
			result.ServiceAccountName = saName
		}
		if result.AutomountServiceAccountToken == nil && policy.ServiceAccountAutomountSAT != nil {
			result.AutomountServiceAccountToken = policy.ServiceAccountAutomountSAT
		}

		rules := make([]rbacv1.PolicyRule, len(policy.Rules))
		for i, rule := range policy.Rules {
			if len(rule.Verbs) == 0 {
				return nil, fmt.Errorf("rbac trait %s: rule %d must specify verbs", saName, i)
			}
			rules[i] = rbacv1.PolicyRule{
				APIGroups:       append([]string(nil), rule.APIGroups...),
				Resources:       append([]string(nil), rule.Resources...),
				ResourceNames:   append([]string(nil), rule.ResourceNames...),
				NonResourceURLs: append([]string(nil), rule.NonResourceURLs...),
				Verbs:           append([]string(nil), rule.Verbs...),
			}
		}

		if policy.ClusterScope {
			role := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:   roleName,
					Labels: utils.CopyStringMap(policy.RoleLabels),
				},
				Rules: rules,
			}
			binding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   bindingName,
					Labels: utils.CopyStringMap(policy.BindingLabels),
				},
				Subjects: []rbacv1.Subject{{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      saName,
					Namespace: namespace,
				}},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     roleName,
				},
			}
			result.AdditionalObjects = append(result.AdditionalObjects, role, binding)
		} else {
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: namespace,
					Labels:    utils.CopyStringMap(policy.RoleLabels),
				},
				Rules: rules,
			}
			binding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: namespace,
					Labels:    utils.CopyStringMap(policy.BindingLabels),
				},
				Subjects: []rbacv1.Subject{{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      saName,
					Namespace: namespace,
				}},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "Role",
					Name:     roleName,
				},
			}
			result.AdditionalObjects = append(result.AdditionalObjects, role, binding)
		}
	}

	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
