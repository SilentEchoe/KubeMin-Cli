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
	Process(ctx *TraitContext) error
}

// TraitContext provides a flexible context for trait processing
type TraitContext struct {
	Component *model.ApplicationComponent
	Workload  interface{}
	TraitData interface{}

	// Resources that will be aggregated
	AdditionalContainers   []corev1.Container
	InitContainers         []corev1.Container
	Volumes                []corev1.Volume
	VolumeMounts           map[int][]corev1.VolumeMount // container index -> volume mounts
	PersistentVolumeClaims []corev1.PersistentVolumeClaim
	ConfigMaps             []corev1.ConfigMap
	Secrets                []corev1.Secret
}

// NewTraitContext creates a new trait context
func NewTraitContext(component *model.ApplicationComponent, workload interface{}, traitData interface{}) *TraitContext {
	return &TraitContext{
		Component:    component,
		Workload:     workload,
		TraitData:    traitData,
		VolumeMounts: make(map[int][]corev1.VolumeMount),
	}
}

// GetPodTemplate gets the PodTemplateSpec from the workload
func (ctx *TraitContext) GetPodTemplate() (*corev1.PodTemplateSpec, error) {
	switch w := ctx.Workload.(type) {
	case *appsv1.Deployment:
		return &w.Spec.Template, nil
	case *appsv1.StatefulSet:
		return &w.Spec.Template, nil
	case *appsv1.DaemonSet:
		return &w.Spec.Template, nil
	default:
		return nil, fmt.Errorf("unsupported workload type: %T", ctx.Workload)
	}
}

// AddContainer adds a container to the additional containers list
func (ctx *TraitContext) AddContainer(container corev1.Container) {
	ctx.AdditionalContainers = append(ctx.AdditionalContainers, container)
}

// AddInitContainer adds an init container
func (ctx *TraitContext) AddInitContainer(container corev1.Container) {
	ctx.InitContainers = append(ctx.InitContainers, container)
}

// AddVolume adds a volume
func (ctx *TraitContext) AddVolume(volume corev1.Volume) {
	ctx.Volumes = append(ctx.Volumes, volume)
}

// AddVolumeMount adds a volume mount to a specific container
func (ctx *TraitContext) AddVolumeMount(containerIndex int, mount corev1.VolumeMount) {
	ctx.VolumeMounts[containerIndex] = append(ctx.VolumeMounts[containerIndex], mount)
}

// AddPVC adds a persistent volume claim
func (ctx *TraitContext) AddPVC(pvc corev1.PersistentVolumeClaim) {
	ctx.PersistentVolumeClaims = append(ctx.PersistentVolumeClaims, pvc)
}

// AddConfigMap adds a config map
func (ctx *TraitContext) AddConfigMap(cm corev1.ConfigMap) {
	ctx.ConfigMaps = append(ctx.ConfigMaps, cm)
}

// AddSecret adds a secret
func (ctx *TraitContext) AddSecret(secret corev1.Secret) {
	ctx.Secrets = append(ctx.Secrets, secret)
}

var (
	// orderedProcessors stores the registered trait processors in the desired execution order.
	orderedProcessors []TraitProcessor
)

// Register registers a new trait processor
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

// GetProcessor retrieves a registered trait processor by name
func GetProcessor(name string) (TraitProcessor, error) {
	for _, p := range orderedProcessors {
		if p.Name() == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no trait processor found for: %s", name)
}

// ApplyTraits applies all registered traits to the component
func ApplyTraits(component *model.ApplicationComponent, workload interface{}) error {
	if component.Traits == nil {
		klog.V(4).Infof("Component %s has no traits to apply.", component.Name)
		return nil
	}

	// Marshal and unmarshal traits
	traitBytes, err := json.Marshal(component.Traits)
	if err != nil {
		return fmt.Errorf("failed to re-marshal traits for component %s: %w", component.Name, err)
	}

	if string(traitBytes) == "{}" || string(traitBytes) == "null" {
		return nil
	}

	var traits model.Traits
	if err := json.Unmarshal(traitBytes, &traits); err != nil {
		return fmt.Errorf("failed to unmarshal traits into concrete type for component %s: %w", component.Name, err)
	}

	val := reflect.ValueOf(traits)

	// Process each trait
	for _, p := range orderedProcessors {
		traitName := p.Name()
		fieldName := strings.ToUpper(traitName[:1]) + traitName[1:]
		field := val.FieldByName(fieldName)

		if !field.IsValid() {
			klog.V(5).Infof("Trait '%s' is registered but not found in the component's traits struct, skipping.", traitName)
			continue
		}

		if field.Kind() == reflect.Slice && field.Len() > 0 {
			klog.V(3).Infof("Applying trait '%s' for component %s.", traitName, component.Name)

			ctx := NewTraitContext(component, workload, field.Interface())
			if err := p.Process(ctx); err != nil {
				return fmt.Errorf("failed to process trait '%s': %w", traitName, err)
			}

			// Aggregate resources from this trait
			if err := AggregateTraitResources(ctx, workload); err != nil {
				return fmt.Errorf("failed to aggregate resources for trait '%s': %w", traitName, err)
			}
		}
	}

	klog.V(2).Infof("Successfully applied traits for component: %s", component.Name)
	return nil
}

// AggregateTraitResources aggregates all resources from a trait context into the workload
func AggregateTraitResources(ctx *TraitContext, workload interface{}) error {
	podTemplate, err := ctx.GetPodTemplate()
	if err != nil {
		return err
	}

	// Add init containers
	podTemplate.Spec.InitContainers = append(podTemplate.Spec.InitContainers, ctx.InitContainers...)

	// Add additional containers
	podTemplate.Spec.Containers = append(podTemplate.Spec.Containers, ctx.AdditionalContainers...)

	// Add volumes
	podTemplate.Spec.Volumes = append(podTemplate.Spec.Volumes, ctx.Volumes...)

	// Add volume mounts to containers
	for containerIndex, mounts := range ctx.VolumeMounts {
		if containerIndex < len(podTemplate.Spec.Containers) {
			podTemplate.Spec.Containers[containerIndex].VolumeMounts = append(
				podTemplate.Spec.Containers[containerIndex].VolumeMounts,
				mounts...,
			)
		}
	}

	// Add PVCs to StatefulSet if applicable
	if statefulSet, ok := workload.(*appsv1.StatefulSet); ok {
		statefulSet.Spec.VolumeClaimTemplates = append(
			statefulSet.Spec.VolumeClaimTemplates,
			ctx.PersistentVolumeClaims...,
		)
	}

	return nil
}
