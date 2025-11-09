package naming

import (
	"fmt"

	"KubeMin-Cli/pkg/apiserver/utils"
)

func buildName(prefix, name, appID, fallback string) string {
	base := utils.NormalizeLowerStrip(name)
	if base == "" {
		base = utils.NormalizeLowerStrip(fallback)
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

func WebServiceName(name, appID string) string {
	name = utils.NormalizeLowerStrip(name)
	if name == "" {
		return utils.NormalizeLowerStrip(appID)
	}
	return fmt.Sprintf("webservice-%s-%s", name, utils.NormalizeLowerStrip(appID))
}

func ServiceName(name, appID string) string     { return buildName("svc", name, appID, "service") }
func IngressName(name, appID string) string     { return buildName("ing", name, appID, "ingress") }
func StoreServerName(name, appID string) string { return buildName("store", name, appID, "store") }
