package job

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"KubeMin-Cli/pkg/apiserver/config"
)

func shareInfoFromLabels(labels map[string]string) (string, config.ShareStrategy) {
	if labels == nil {
		return "", ""
	}
	shareName := strings.TrimSpace(labels[config.LabelShareName])
	if shareName == "" {
		return "", ""
	}
	rawStrategy := strings.TrimSpace(labels[config.LabelShareStrategy])
	return shareName, normalizeShareStrategy(rawStrategy)
}

func normalizeShareStrategy(strategy string) config.ShareStrategy {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "":
		return config.ShareStrategyDefault
	case string(config.ShareStrategyDefault):
		return config.ShareStrategyDefault
	case string(config.ShareStrategyIgnore):
		return config.ShareStrategyIgnore
	case string(config.ShareStrategyForce):
		return config.ShareStrategyForce
	default:
		return config.ShareStrategyDefault
	}
}

func sharedListOptions(shareName string) metav1.ListOptions {
	selector := labels.Set{config.LabelShareName: shareName}.String()
	return metav1.ListOptions{LabelSelector: selector}
}

func hasSharedResources(ctx context.Context, shareName string, listFn func(context.Context, metav1.ListOptions) (int, error)) (bool, error) {
	if shareName == "" {
		return false, nil
	}
	count, err := listFn(ctx, sharedListOptions(shareName))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
