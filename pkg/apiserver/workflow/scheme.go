package workflow

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Builder struct {
	GroupVersion schema.GroupVersion
	runtime.SchemeBuilder
}
