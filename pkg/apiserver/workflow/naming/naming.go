package naming

import (
	"fmt"
	"strings"

	"KubeMin-Cli/pkg/apiserver/utils"
)

const (
	maxResourceNameLength = 63
	defaultComponentName  = "component"
	defaultAppSegment     = "app"
)

// WebServiceName builds a deterministic deployment name for stateless components.
func WebServiceName(name, appID string) string {
	return buildResourceName("deploy", name, appID)
}

// SharedWebServiceName builds a deterministic deployment name without app scoping.
func SharedWebServiceName(name string) string {
	return buildResourceNameNoApp("deploy", name)
}

// ServiceName builds a deterministic Service name for components.
func ServiceName(name, appID string) string {
	return buildResourceName("svc", name, appID)
}

// SharedServiceName builds a deterministic Service name without app scoping.
func SharedServiceName(name string) string {
	return buildResourceNameNoApp("svc", name)
}

// StoreServerName builds a StatefulSet name for store components.
func StoreServerName(name, appID string) string {
	return buildResourceName("store", name, appID)
}

// SharedStoreServerName builds a StatefulSet name without app scoping.
func SharedStoreServerName(name string) string {
	return buildResourceNameNoApp("store", name)
}

// IngressName builds an ingress resource name tied to the component/app pair.
func IngressName(name, appID string) string {
	return buildResourceName("ing", name, appID)
}

// SharedIngressName builds an ingress resource name without app scoping.
func SharedIngressName(name string) string {
	return buildResourceNameNoApp("ing", name)
}

// PVCName formats PVC names as pvc-<traitName>-<appID> with normalized segments.
func PVCName(traitName, appID string) string {
	name := utils.NormalizeLowerStrip(traitName)
	if name == "" {
		name = "data"
	}
	suffix := utils.NormalizeLowerStrip(appID)
	if suffix == "" {
		return fmt.Sprintf("pvc-%s", name)
	}
	return fmt.Sprintf("pvc-%s-%s", name, suffix)
}

// SharedPVCName formats PVC names as pvc-<traitName> with normalized segments.
func SharedPVCName(traitName string) string {
	name := utils.NormalizeLowerStrip(traitName)
	if name == "" {
		name = "data"
	}
	return fmt.Sprintf("pvc-%s", name)
}

func buildResourceName(prefix, componentName, appID string) string {
	component := normalizeSegment(componentName, defaultComponentName)
	app := normalizeSegment(appID, defaultAppSegment)

	var parts []string
	if prefix != "" {
		parts = append(parts, prefix)
	}
	if component != "" {
		parts = append(parts, component)
	}
	if app != "" {
		parts = append(parts, app)
	}

	result := utils.ToRFC1123Name(strings.Join(parts, "-"))
	if len(result) > maxResourceNameLength {
		result = strings.Trim(result[:maxResourceNameLength], "-")
	}
	if result == "" {
		result = prefix
		if result == "" {
			result = defaultComponentName
		}
	}
	return result
}

func buildResourceNameNoApp(prefix, componentName string) string {
	component := normalizeSegment(componentName, defaultComponentName)

	var parts []string
	if prefix != "" {
		parts = append(parts, prefix)
	}
	if component != "" {
		parts = append(parts, component)
	}

	result := utils.ToRFC1123Name(strings.Join(parts, "-"))
	if len(result) > maxResourceNameLength {
		result = strings.Trim(result[:maxResourceNameLength], "-")
	}
	if result == "" {
		result = prefix
		if result == "" {
			result = defaultComponentName
		}
	}
	return result
}

func normalizeSegment(value, fallback string) string {
	normalized := utils.NormalizeLowerStrip(value)
	if normalized == "" {
		return fallback
	}
	return normalized
}
