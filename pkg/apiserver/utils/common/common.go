package common

import (
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

var (
	Scheme = k8sruntime.NewScheme()
)
