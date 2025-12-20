package workflow

import (
	"encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
	"KubeMin-Cli/pkg/apiserver/utils"
)

type shareConfig struct {
	Strategy config.ShareStrategy
	Name     string
}

func (s shareConfig) enabled() bool {
	return s.Strategy != ""
}

func (s shareConfig) ignore() bool {
	return s.Strategy == config.ShareStrategyIgnore
}

func shareConfigForComponent(component *model.ApplicationComponent) shareConfig {
	if component == nil || component.Traits == nil {
		return shareConfig{}
	}

	traitBytes, err := json.Marshal(component.Traits)
	if err != nil {
		klog.Errorf("failed to marshal traits for share lookup: %v", err)
		return shareConfig{}
	}

	if string(traitBytes) == "{}" || string(traitBytes) == "null" {
		return shareConfig{}
	}

	var traits spec.Traits
	if err := json.Unmarshal(traitBytes, &traits); err != nil {
		klog.Errorf("failed to unmarshal traits for share lookup: %v", err)
		return shareConfig{}
	}

	if traits.Share == nil {
		return shareConfig{}
	}

	strategy := normalizeShareStrategy(traits.Share.Strategy)
	shareName := shareNameForComponent(component)
	return shareConfig{
		Strategy: strategy,
		Name:     shareName,
	}
}

func normalizeShareStrategy(strategy string) config.ShareStrategy {
	normalized := strings.ToLower(strings.TrimSpace(strategy))
	switch normalized {
	case "":
		return config.ShareStrategyDefault
	case string(config.ShareStrategyDefault):
		return config.ShareStrategyDefault
	case string(config.ShareStrategyIgnore):
		return config.ShareStrategyIgnore
	case string(config.ShareStrategyForce):
		return config.ShareStrategyForce
	default:
		klog.Warningf("unknown share strategy %q, falling back to default", strategy)
		return config.ShareStrategyDefault
	}
}

func shareNameForComponent(component *model.ApplicationComponent) string {
	if component == nil {
		return ""
	}
	baseName := component.Name
	if baseName == "" {
		baseName = "shared"
	}
	kind := string(component.ComponentType)
	if kind != "" {
		baseName = fmt.Sprintf("%s-%s", baseName, kind)
	}
	baseName = utils.ToRFC1123Name(baseName)
	if len(baseName) > 63 {
		baseName = strings.Trim(baseName[:63], "-")
	}
	if baseName == "" {
		baseName = "shared"
	}
	return baseName
}

func applyShareLabels(labels map[string]string, share shareConfig) map[string]string {
	if !share.enabled() {
		return labels
	}
	if labels == nil {
		labels = make(map[string]string, 2)
	}
	labels[config.LabelShareName] = share.Name
	labels[config.LabelShareStrategy] = string(share.Strategy)
	return labels
}

func applyShareLabelsToObject(obj metav1.Object, share shareConfig) {
	if obj == nil {
		return
	}
	obj.SetLabels(applyShareLabels(obj.GetLabels(), share))
}

func applyShareLabelsToJobInfo(jobInfo interface{}, share shareConfig) {
	if !share.enabled() {
		return
	}
	switch info := jobInfo.(type) {
	case metav1.Object:
		applyShareLabelsToObject(info, share)
	case *applyv1.ServiceApplyConfiguration:
		info.Labels = applyShareLabels(info.Labels, share)
	case *model.ConfigMapInput:
		info.Labels = applyShareLabels(info.Labels, share)
	case *model.SecretInput:
		info.Labels = applyShareLabels(info.Labels, share)
	}
}
