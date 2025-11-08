package naming

import (
	"fmt"

	"KubeMin-Cli/pkg/apiserver/utils"
)

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
