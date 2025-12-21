package config

import "strings"

// NormalizeShareStrategy returns a normalized strategy and whether the input is known.
func NormalizeShareStrategy(strategy string) (ShareStrategy, bool) {
	normalized := strings.ToLower(strings.TrimSpace(strategy))
	switch normalized {
	case "":
		return ShareStrategyDefault, true
	case string(ShareStrategyDefault):
		return ShareStrategyDefault, true
	case string(ShareStrategyIgnore):
		return ShareStrategyIgnore, true
	case string(ShareStrategyForce):
		return ShareStrategyForce, true
	default:
		return ShareStrategyDefault, false
	}
}
