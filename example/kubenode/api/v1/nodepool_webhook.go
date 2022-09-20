package v1

import "regexp"

var (
	keyReg = regexp.MustCompile(`^node-pool.lailin.xyz/*[a-zA-z0-9]*$`)
)
