package v1

import (
	"regexp"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	nodePoolLog = logf.Log.WithName("nodepool-resource")
	keyReg      = regexp.MustCompile(`^node-pool.lailin.xyz/*[a-zA-z0-9]*$`)
)
