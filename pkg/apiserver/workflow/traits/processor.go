package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

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
	// orderedProcessors stores the registered trait processors in the desired execution order.
	orderedProcessors []TraitProcessor
)

// Register registers a new trait processor, appending it to the execution list.
func Register(p TraitProcessor) {
	name := p.Name()
	for _, existing := range orderedProcessors {
		if existing.Name() == name {
			klog.Fatalf("Trait processor already registered: %s", name)
		}
	}
	klog.V(4).Infof("Registering trait processor: %s", name)
	orderedProcessors = append(orderedProcessors, p)
}

// GetProcessor retrieves a registered trait processor by name.
func GetProcessor(name string) (TraitProcessor, error) {
	for _, p := range orderedProcessors {
		if p.Name() == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no trait processor found for: %s", name)
}

// ApplyTraits iterates through the registered traits and applies them to the workload if they are defined in the component.
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

	// Use reflection to get the value of the traits struct
	val := reflect.ValueOf(traits)

	// Iterate through the processors in their registration order.
	for _, p := range orderedProcessors {
		traitName := p.Name()
		// Capitalize first letter for struct field name, e.g., "sidecar" -> "Sidecar"
		fieldName := strings.ToUpper(traitName[:1]) + traitName[1:]
		field := val.FieldByName(fieldName)

		if !field.IsValid() {
			// This case should ideally not happen if naming conventions are followed.
			klog.V(5).Infof("Trait '%s' is registered but not found in the component's traits struct, skipping.", traitName)
			continue
		}

		// Check if the trait data is present (e.g., slice is not empty).
		if field.Kind() == reflect.Slice && field.Len() > 0 {
			klog.V(3).Infof("Applying trait '%s' for component %s.", traitName, component.Name)
			if err := p.Process(workload, field.Interface(), component); err != nil {
				return fmt.Errorf("failed to process trait '%s': %w", traitName, err)
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
