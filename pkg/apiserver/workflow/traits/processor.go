package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// TraitProcessor is the interface for all trait processors.
type TraitProcessor interface {
	Name() string
	Process(workload interface{}, traitData interface{}, component *model.ApplicationComponent) error
}

var (
	traitRegistry = make(map[string]TraitProcessor)
)

// Register registers a new trait processor.
func Register(p TraitProcessor) {
	name := p.Name()
	if _, found := traitRegistry[name]; found {
		klog.Fatalf("Trait processor already registered: %s", name)
	}
	klog.V(4).Infof("Registering trait processor: %s", name)
	traitRegistry[name] = p
}

// GetProcessor retrieves a registered trait processor by name.
func GetProcessor(name string) (TraitProcessor, error) {
	p, found := traitRegistry[name]
	if !found {
		return nil, fmt.Errorf("no trait processor found for: %s", name)
	}
	return p, nil
}

// ApplyTraits iterates through the traits of a component and applies them to the workload.
func ApplyTraits(component *model.ApplicationComponent, workload interface{}) error {
	if component.Traits == nil {
		klog.V(4).Infof("Component %s has no traits to apply.", component.Name)
		return nil
	}

	// First, marshal the generic *JSONStruct back into a byte slice.
	traitBytes, err := json.Marshal(component.Traits)
	if err != nil {
		return fmt.Errorf("failed to re-marshal traits for component %s: %w", component.Name, err)
	}

	// An empty traits field might be represented as "{}" or "null".
	if string(traitBytes) == "{}" || string(traitBytes) == "null" {
		return nil
	}

	// Second, unmarshal the byte slice into our concrete model.Traits struct.
	var traits model.Traits
	if err := json.Unmarshal(traitBytes, &traits); err != nil {
		return fmt.Errorf("failed to unmarshal traits into concrete type for component %s: %w", component.Name, err)
	}

	// The order of trait application can be important.
	// We apply storage first as other traits might need the created volumes.
	traitProcessingOrder := []struct {
		Name      string
		TraitData interface{}
		Enabled   bool
	}{
		{"storage", traits.Storage, len(traits.Storage) > 0},
		{"sidecar", traits.Sidecar, len(traits.Sidecar) > 0},
		{"config", traits.Config, len(traits.Config) > 0},
		{"secret", traits.Secret, len(traits.Secret) > 0},
	}

	for _, t := range traitProcessingOrder {
		if t.Enabled {
			p, err := GetProcessor(t.Name)
			if err != nil {
				klog.Warningf("No processor found for enabled trait '%s', skipping.", t.Name)
				continue
			}
			if err := p.Process(workload, t.TraitData, component); err != nil {
				return fmt.Errorf("failed to process trait '%s': %w", t.Name, err)
			}
		}
	}

	klog.V(2).Infof("Successfully applied traits for component: %s", component.Name)
	return nil
}

// GetPodTemplateSpec is a helper function to get the workload's PodTemplateSpec.
func GetPodTemplateSpec(workload interface{}) (*corev1.PodTemplateSpec, error) {
	switch w := workload.(type) {
	case *appsv1.Deployment:
		return &w.Spec.Template, nil
	case *appsv1.StatefulSet:
		return &w.Spec.Template, nil
	case *appsv1.DaemonSet:
		return &w.Spec.Template, nil
	default:
		return nil, fmt.Errorf("unsupported workload type: %T", workload)
	}
}