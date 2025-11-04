package traits

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"KubeMin-Cli/pkg/apiserver/domain/model"
	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
)

func TestRBACProcessor_NamespaceRole(t *testing.T) {
	p := &RBACProcessor{}
	component := &model.ApplicationComponent{
		Name:      "backend",
		Namespace: "demo",
	}
	policies := []spec.RBACPolicySpec{
		{
			ServiceAccount: "custom-sa",
			Rules: []spec.RBACRuleSpec{
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get", "list"},
				},
			},
			ServiceAccountLabels: map[string]string{"app": "demo"},
			RoleLabels:           map[string]string{"role": "reader"},
			BindingLabels:        map[string]string{"binding": "reader"},
		},
	}

	res, err := p.Process(&TraitContext{Component: component, TraitData: policies})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.AdditionalObjects, 3)

	sa, ok := res.AdditionalObjects[0].(*corev1.ServiceAccount)
	require.True(t, ok)
	require.Equal(t, "custom-sa", sa.Name)
	require.Equal(t, "demo", sa.Namespace)
	require.Equal(t, "demo", sa.Labels["app"])

	role, ok := res.AdditionalObjects[1].(*rbacv1.Role)
	require.True(t, ok)
	require.Equal(t, "custom-sa-role", role.Name)
	require.Equal(t, "demo", role.Namespace)
	require.Equal(t, 1, len(role.Rules))
	require.Equal(t, []string{"get", "list"}, role.Rules[0].Verbs)
	require.Equal(t, "reader", role.Labels["role"])

	binding, ok := res.AdditionalObjects[2].(*rbacv1.RoleBinding)
	require.True(t, ok)
	require.Equal(t, "custom-sa-binding", binding.Name)
	require.Equal(t, "demo", binding.Namespace)
	require.Equal(t, "reader", binding.Labels["binding"])
	require.Equal(t, 1, len(binding.Subjects))
	require.Equal(t, "custom-sa", binding.Subjects[0].Name)
	require.Equal(t, "Role", binding.RoleRef.Kind)
	require.Equal(t, "custom-sa-role", binding.RoleRef.Name)
}

func TestRBACProcessor_ClusterScope(t *testing.T) {
	p := &RBACProcessor{}
	component := &model.ApplicationComponent{
		Name:      "controller",
		Namespace: "system",
	}
	policies := []spec.RBACPolicySpec{
		{
			ClusterScope: true,
			Rules: []spec.RBACRuleSpec{
				{
					APIGroups: []string{"apps"},
					Resources: []string{"deployments"},
					Verbs:     []string{"update"},
				},
			},
		},
	}

	res, err := p.Process(&TraitContext{Component: component, TraitData: policies})
	require.NoError(t, err)
	require.Len(t, res.AdditionalObjects, 3)

	sa := res.AdditionalObjects[0].(*corev1.ServiceAccount)
	require.Equal(t, "controller-sa", sa.Name)
	require.Equal(t, "system", sa.Namespace)

	clusterRole := res.AdditionalObjects[1].(*rbacv1.ClusterRole)
	require.Equal(t, "controller-sa-role", clusterRole.Name)
	require.Equal(t, []string{"apps"}, clusterRole.Rules[0].APIGroups)

	clusterBinding := res.AdditionalObjects[2].(*rbacv1.ClusterRoleBinding)
	require.Equal(t, "controller-sa-binding", clusterBinding.Name)
	require.Equal(t, "ClusterRole", clusterBinding.RoleRef.Kind)
	require.Equal(t, "controller-sa-role", clusterBinding.RoleRef.Name)
	require.Equal(t, "system", clusterBinding.Subjects[0].Namespace)
}
