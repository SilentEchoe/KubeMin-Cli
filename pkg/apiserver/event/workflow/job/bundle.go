package job

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
	"KubeMin-Cli/pkg/apiserver/utils"
)

const bundleAppIDPrefix = "bundle:"

func bundleAppID(bundleName string) string {
	name := strings.TrimSpace(bundleName)
	if name == "" {
		name = "bundle"
	}
	return bundleAppIDPrefix + name
}

func bundleFromComponent(component *model.ApplicationComponent) *spec.BundleTraitSpec {
	if component == nil {
		return nil
	}
	traits := ParseTraits(component.Traits)
	if traits.Bundle == nil || strings.TrimSpace(traits.Bundle.Name) == "" {
		return nil
	}
	return traits.Bundle
}

func bundleFromJob(job *model.JobTask) *spec.BundleTraitSpec {
	if job == nil {
		return nil
	}
	if job.Bundle == nil || strings.TrimSpace(job.Bundle.Name) == "" {
		return nil
	}
	return job.Bundle
}

func bundleAnchorSpec(bundle *spec.BundleTraitSpec) (kind, name string) {
	if bundle == nil {
		return "", ""
	}
	kind = strings.TrimSpace(bundle.Anchor.Kind)
	if kind == "" {
		kind = "ConfigMap"
	}
	name = strings.TrimSpace(bundle.Anchor.Name)
	if name == "" && strings.EqualFold(kind, "ConfigMap") {
		name = defaultBundleAnchorName(bundle.Name)
	}
	return kind, name
}

func BundleAnchor(bundle *spec.BundleTraitSpec) (kind, name string) {
	return bundleAnchorSpec(bundle)
}

func defaultBundleAnchorName(bundleName string) string {
	suffix := utils.NormalizeLowerStrip(bundleName)
	if suffix == "" {
		suffix = "bundle"
	}
	base := fmt.Sprintf("kubemin-bundle-%s", suffix)
	normalized := utils.ToRFC1123Name(base)
	if len(normalized) > 63 {
		normalized = strings.Trim(normalized[:63], "-")
	}
	if normalized == "" {
		normalized = "kubemin-bundle"
	}
	return normalized
}

func bundleOwns(labels map[string]string, bundleName string) bool {
	if labels == nil {
		return false
	}
	return labels[config.LabelBundle] == bundleName
}

func EnsureBundleLabels(obj metav1.Object, bundleName, memberName string) {
	if obj == nil {
		return
	}
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string, 4)
	}
	labels[config.LabelAppID] = bundleAppID(bundleName)
	labels[config.LabelBundle] = bundleName
	if memberName != "" {
		labels[config.LabelBundleMember] = memberName
	}
	obj.SetLabels(labels)
}

func shouldSkipBundleJob(ctx context.Context, client kubernetes.Interface, namespace string, bundle *spec.BundleTraitSpec) (bool, error) {
	if client == nil || bundle == nil {
		return false, nil
	}
	if namespace == "" {
		namespace = config.DefaultNamespace
	}
	kind, name := bundleAnchorSpec(bundle)
	if name == "" {
		return false, fmt.Errorf("bundle %q has empty anchor name", bundle.Name)
	}

	var (
		labels map[string]string
		err    error
	)

	switch {
	case strings.EqualFold(kind, "ConfigMap"):
		var cm *corev1.ConfigMap
		cm, err = client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil && cm != nil {
			labels = cm.Labels
		}
	case strings.EqualFold(kind, "Deployment"):
		var deploy *appsv1.Deployment
		deploy, err = client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil && deploy != nil {
			labels = deploy.Labels
		}
	default:
		return false, fmt.Errorf("unsupported bundle anchor kind %q (bundle=%q)", kind, bundle.Name)
	}

	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get bundle anchor %s/%s (%s): %w", namespace, name, kind, err)
	}

	if !bundleOwns(labels, bundle.Name) {
		return false, fmt.Errorf("bundle anchor %s/%s exists but does not match bundle=%q", namespace, name, bundle.Name)
	}
	return true, nil
}
