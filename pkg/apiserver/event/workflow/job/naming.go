package job

import (
	"fmt"

	"KubeMin-Cli/pkg/apiserver/utils"
)

// buildName builds a standardized RFC1123-compliant resource name as:
//
//	<prefix>-<base>-<suffix>
//
// where base comes from component name and suffix comes from appID. If name
// is empty, fallbackBase is used instead. If appID is empty, the suffix part
// is omitted.
func buildName(prefix, name, appID, fallbackBase string) string {
	base := utils.NormalizeLowerStrip(name)
	if base == "" {
		base = utils.NormalizeLowerStrip(fallbackBase)
	}
	suffix := utils.NormalizeLowerStrip(appID)

	switch {
	case prefix != "" && base != "" && suffix != "":
		return fmt.Sprintf("%s-%s-%s", prefix, base, suffix)
	case prefix != "" && base != "":
		return fmt.Sprintf("%s-%s", prefix, base)
	case base != "" && suffix != "":
		return fmt.Sprintf("%s-%s", base, suffix)
	default:
		return base
	}
}

func buildDeploymentName(name, appID string) string {
	return buildName("webservice", name, appID, "webservice")
}

func buildServiceName(name, appID string) string {
	return buildName("svc", name, appID, "service")
}

func buildIngressName(name, appID string) string {
	return buildName("ing", name, appID, "ingress")
}

func buildPVCName(name, appID string) string {
	return buildName("pvc", name, appID, "pvc")
}

func buildConfigMapName(name, appID string) string {
	return buildName("cm", name, appID, "config")
}

func buildSecretName(name, appID string) string {
	return buildName("secret", name, appID, "secret")
}

func buildStoreSeverName(name, appID string) string {
	return buildName("store", name, appID, "store")
}
