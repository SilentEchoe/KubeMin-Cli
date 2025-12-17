package job

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
)

func TestDeployConfigMapJobCtlBundleSkipsWhenAnchorExists(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	bundle := &spec.BundleTraitSpec{
		Name: "shared",
		Anchor: spec.BundleAnchorSpec{
			Kind: "ConfigMap",
			Name: "kubemin-bundle-shared",
		},
	}

	anchor := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundle.Anchor.Name,
			Namespace: "default",
			Labels: map[string]string{
				config.LabelBundle: bundle.Name,
				config.LabelAppID:  bundleAppID(bundle.Name),
			},
		},
	}
	_, err := client.CoreV1().ConfigMaps("default").Create(ctx, anchor, metav1.CreateOptions{})
	require.NoError(t, err)

	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-config",
			Namespace: "default",
			Labels: map[string]string{
				config.LabelBundle: bundle.Name,
				config.LabelAppID:  bundleAppID(bundle.Name),
			},
		},
		Data: map[string]string{"key": "old"},
	}
	_, err = client.CoreV1().ConfigMaps("default").Create(ctx, existing, metav1.CreateOptions{})
	require.NoError(t, err)

	desired := existing.DeepCopy()
	desired.Data = map[string]string{"key": "new"}

	jobTask := &model.JobTask{
		Name:      "app-config",
		Namespace: "default",
		AppID:     "app-1",
		JobType:   string(config.JobDeployConfigMap),
		JobInfo:   desired,
		Bundle:    bundle,
	}

	ctl := NewDeployConfigMapJobCtl(jobTask, client, nil, func() {})
	require.NoError(t, ctl.Run(ctx))
	require.Equal(t, config.StatusSkipped, jobTask.Status)

	got, err := client.CoreV1().ConfigMaps("default").Get(ctx, "app-config", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"key": "old"}, got.Data)
}

func TestDeployConfigMapJobCtlBundleSkipsUpdateWhenResourceExists(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	bundle := &spec.BundleTraitSpec{
		Name: "shared",
		Anchor: spec.BundleAnchorSpec{
			Kind: "ConfigMap",
			Name: "kubemin-bundle-shared",
		},
	}

	existing := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-config",
			Namespace: "default",
			Labels: map[string]string{
				config.LabelBundle: bundle.Name,
				config.LabelAppID:  bundleAppID(bundle.Name),
			},
		},
		Data: map[string]string{"key": "old"},
	}
	_, err := client.CoreV1().ConfigMaps("default").Create(ctx, existing, metav1.CreateOptions{})
	require.NoError(t, err)

	desired := existing.DeepCopy()
	desired.Data = map[string]string{"key": "new"}

	jobTask := &model.JobTask{
		Name:      "app-config",
		Namespace: "default",
		AppID:     "app-1",
		JobType:   string(config.JobDeployConfigMap),
		JobInfo:   desired,
		Bundle:    bundle,
	}

	ctl := NewDeployConfigMapJobCtl(jobTask, client, nil, func() {})
	require.NoError(t, ctl.Run(ctx))
	require.Equal(t, config.StatusCompleted, jobTask.Status)

	got, err := client.CoreV1().ConfigMaps("default").Get(ctx, "app-config", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"key": "old"}, got.Data)
}

func TestBuildLabelsAndSharedNamesForBundle(t *testing.T) {
	traits := model.JSONStruct{
		"bundle": map[string]interface{}{
			"name": "shared",
		},
	}

	component := &model.ApplicationComponent{
		Name:      "redis",
		Namespace: "default",
		AppID:     "app-1",
		ID:        42,
		Traits:    &traits,
	}

	labels := BuildLabels(component, &model.Properties{})
	require.Equal(t, "shared", labels[config.LabelBundle])
	require.Equal(t, "redis", labels[config.LabelBundleMember])
	require.Equal(t, bundleAppID("shared"), labels[config.LabelAppID])
	require.Empty(t, labels[config.LabelComponentID])
	require.Empty(t, labels[config.LabelComponentName])

	result := GenerateWebService(component, &model.Properties{})
	require.NotNil(t, result)
	deploy, ok := result.Service.(*appsv1.Deployment)
	require.True(t, ok)
	require.Equal(t, "deploy-redis", deploy.Name)
	require.Equal(t, "shared", deploy.Labels[config.LabelBundle])
	require.Equal(t, bundleAppID("shared"), deploy.Labels[config.LabelAppID])
}

func TestShouldSkipBundleJobErrorsOnMismatchedAnchor(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()

	bundle := &spec.BundleTraitSpec{
		Name: "shared",
		Anchor: spec.BundleAnchorSpec{
			Kind: "ConfigMap",
			Name: "kubemin-bundle-shared",
		},
	}

	anchor := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bundle.Anchor.Name,
			Namespace: "default",
			Labels: map[string]string{
				config.LabelBundle: "other",
				config.LabelAppID:  bundleAppID("other"),
			},
		},
	}
	_, err := client.CoreV1().ConfigMaps("default").Create(ctx, anchor, metav1.CreateOptions{})
	require.NoError(t, err)

	skip, err := shouldSkipBundleJob(ctx, client, "default", bundle)
	require.Error(t, err)
	require.False(t, skip)
}
