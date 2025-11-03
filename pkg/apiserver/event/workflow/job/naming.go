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

func buildWebServiceName(name, appID string) string {
	name = utils.NormalizeLowerStrip(name)
	if name == "" {
		return utils.NormalizeLowerStrip(appID)
	}
	return fmt.Sprintf("webservice-%s-%s", name, utils.NormalizeLowerStrip(appID))
}

func buildServiceName(name, appID string) string   { return buildName("svc", name, appID, "service") }
func buildIngressName(name, appID string) string   { return buildName("ing", name, appID, "ingress") }
func buildPVCName(name, appID string) string       { return buildName("pvc", name, appID, "pvc") }
func buildConfigMapName(name, appID string) string { return buildName("cm", name, appID, "config") }
func buildSecretName(name, appID string) string    { return buildName("secret", name, appID, "secret") }
func buildStoreSeverName(name, appID string) string {
	return buildName("store", name, appID, "store")
}

// BuildIngressName returns a normalized ingress resource name for the given component/app.
func BuildIngressName(name, appID string) string { return buildIngressName(name, appID) }

// BuildPVCName returns a normalized PVC resource name for the given component/app.
func BuildPVCName(name, appID string) string { return buildPVCName(name, appID) }

// BuildConfigMapName returns a normalized ConfigMap resource name for the given component/app.
func BuildConfigMapName(name, appID string) string { return buildConfigMapName(name, appID) }

// BuildSecretName returns a normalized Secret resource name for the given component/app.
func BuildSecretName(name, appID string) string { return buildSecretName(name, appID) }

// BuildServiceName returns a normalized Service resource name for the given component/app.
func BuildServiceName(name, appID string) string { return buildServiceName(name, appID) }

// BuildStoreServerName returns a normalized store workload name for the given component/app.
func BuildStoreServerName(name, appID string) string { return buildStoreSeverName(name, appID) }
