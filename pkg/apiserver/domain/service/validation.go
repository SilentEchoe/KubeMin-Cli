package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"kubemin-cli/pkg/apiserver/config"
	"kubemin-cli/pkg/apiserver/domain/repository"
	"kubemin-cli/pkg/apiserver/domain/spec"
	apisv1 "kubemin-cli/pkg/apiserver/interfaces/api/dto/v1"
)

var (
	// DNS-1123 subdomain pattern: lowercase alphanumeric, may contain hyphens
	// Must start and end with alphanumeric character
	nameRegexp = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	// Kubernetes resource quantity pattern for storage size validation
	storageQuantityRegexp = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(Ki|Mi|Gi|Ti|Pi|Ei|K|M|G|T|P|E)?$`)

	// Valid storage types
	validStorageTypes = map[string]bool{
		config.StorageTypePersistent: true,
		config.StorageTypeEphemeral:  true,
		config.StorageTypeConfig:     true,
		config.StorageTypeSecret:     true,
	}

	// Valid probe types
	validProbeTypes = map[string]bool{
		"liveness":  true,
		"readiness": true,
		"startup":   true,
	}

	// Valid component types
	validComponentTypes = map[config.JobType]bool{
		config.ServerJob: true,
		config.StoreJob:  true,
		config.ConfJob:   true,
		config.SecretJob: true,
	}

	// Valid workflow modes
	validWorkflowModes = map[string]bool{
		string(config.WorkflowModeStepByStep): true,
		string(config.WorkflowModeDAG):        true,
		"":                                    true, // Empty is allowed (defaults to StepByStep)
	}

	// Valid env_from types
	validEnvFromTypes = map[string]bool{
		"secret":    true,
		"configMap": true,
	}
)

const (
	minNameLength = 2
	maxNameLength = 63 // DNS-1123 subdomain max length
)

// ValidationService provides validation capabilities for applications and workflows
type ValidationService interface {
	// TryApplication validates an application creation request without actually creating it
	TryApplication(ctx context.Context, req apisv1.CreateApplicationsRequest) *apisv1.TryApplicationResponse

	// TryWorkflow validates a workflow update request against existing components
	TryWorkflow(ctx context.Context, appID string, req apisv1.TryWorkflowRequest) *apisv1.TryWorkflowResponse
}

type validationServiceImpl struct {
	AppRepo       repository.ApplicationRepository `inject:""`
	ComponentRepo repository.ComponentRepository   `inject:""`
}

// NewValidationService creates a new ValidationService instance
func NewValidationService() ValidationService {
	return &validationServiceImpl{}
}

// TryApplication validates an application creation request
func (v *validationServiceImpl) TryApplication(ctx context.Context, req apisv1.CreateApplicationsRequest) *apisv1.TryApplicationResponse {
	var errors []apisv1.ValidationError

	// 1. Validate application name
	errors = append(errors, v.validateName(req.Name, "name")...)

	// 2. Validate components
	componentNames := make(map[string]bool)
	for i, comp := range req.Component {
		fieldPrefix := fmt.Sprintf("component[%d]", i)
		errors = append(errors, v.validateComponent(comp, fieldPrefix, componentNames)...)
	}

	// 3. Validate workflow steps and component references
	errors = append(errors, v.validateWorkflowSteps(req.WorkflowSteps, componentNames, "workflow")...)

	return &apisv1.TryApplicationResponse{
		Valid:  len(errors) == 0,
		Errors: errors,
	}
}

// TryWorkflow validates a workflow update request against existing components
func (v *validationServiceImpl) TryWorkflow(ctx context.Context, appID string, req apisv1.TryWorkflowRequest) *apisv1.TryWorkflowResponse {
	var errors []apisv1.ValidationError

	// 1. Validate workflow name if provided
	if req.Name != "" {
		errors = append(errors, v.validateName(req.Name, "name")...)
	}

	// 2. Get existing components for the application
	componentNames := make(map[string]bool)
	if appID != "" {
		components, err := v.ComponentRepo.FindByAppID(ctx, appID)
		if err == nil {
			for _, comp := range components {
				if comp != nil {
					componentNames[strings.ToLower(comp.Name)] = true
				}
			}
		}
	}

	// 3. Validate workflow steps
	if len(req.Workflow) == 0 {
		errors = append(errors, apisv1.ValidationError{
			Field:   "workflow",
			Code:    apisv1.ErrCodeMissingRequiredField,
			Message: "workflow must contain at least one step",
		})
	} else {
		errors = append(errors, v.validateWorkflowSteps(req.Workflow, componentNames, "workflow")...)
	}

	return &apisv1.TryWorkflowResponse{
		Valid:  len(errors) == 0,
		Errors: errors,
	}
}

// validateName validates a name against DNS-1123 subdomain rules
func (v *validationServiceImpl) validateName(name, field string) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	if name == "" {
		errors = append(errors, apisv1.ValidationError{
			Field:   field,
			Code:    apisv1.ErrCodeMissingRequiredField,
			Message: fmt.Sprintf("%s is required", field),
		})
		return errors
	}

	if len(name) < minNameLength {
		errors = append(errors, apisv1.ValidationError{
			Field:   field,
			Code:    apisv1.ErrCodeNameTooShort,
			Message: fmt.Sprintf("%s must be at least %d characters", field, minNameLength),
		})
	}

	if len(name) > maxNameLength {
		errors = append(errors, apisv1.ValidationError{
			Field:   field,
			Code:    apisv1.ErrCodeNameTooLong,
			Message: fmt.Sprintf("%s must be at most %d characters", field, maxNameLength),
		})
	}

	// Convert to lowercase for validation (names should be lowercase)
	lowerName := strings.ToLower(name)
	if !nameRegexp.MatchString(lowerName) {
		errors = append(errors, apisv1.ValidationError{
			Field:   field,
			Code:    apisv1.ErrCodeInvalidNameFormat,
			Message: fmt.Sprintf("%s must match DNS-1123 subdomain (lowercase alphanumeric, may contain hyphens, must start and end with alphanumeric)", field),
		})
	}

	return errors
}

// validateComponent validates a single component configuration
func (v *validationServiceImpl) validateComponent(comp apisv1.CreateComponentRequest, fieldPrefix string, componentNames map[string]bool) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	// Validate component name
	nameField := fmt.Sprintf("%s.name", fieldPrefix)
	errors = append(errors, v.validateName(comp.Name, nameField)...)

	// Check for duplicate component names
	lowerName := strings.ToLower(comp.Name)
	if componentNames[lowerName] {
		errors = append(errors, apisv1.ValidationError{
			Field:   nameField,
			Code:    apisv1.ErrCodeDuplicateComponent,
			Message: fmt.Sprintf("duplicate component name: %s", comp.Name),
		})
	} else {
		componentNames[lowerName] = true
	}

	// Validate component type
	if !validComponentTypes[comp.ComponentType] {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.type", fieldPrefix),
			Code:    apisv1.ErrCodeInvalidComponentType,
			Message: fmt.Sprintf("invalid component type: %s, must be one of: webservice, store, config, secret", comp.ComponentType),
		})
	}

	// Validate image requirement for webservice and store types
	if (comp.ComponentType == config.ServerJob || comp.ComponentType == config.StoreJob) && comp.Image == "" {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.image", fieldPrefix),
			Code:    apisv1.ErrCodeMissingImage,
			Message: "image is required for webservice and store component types",
		})
	}

	// Validate traits
	traitsField := fmt.Sprintf("%s.traits", fieldPrefix)
	errors = append(errors, v.validateTraits(comp.Traits, traitsField, false)...)

	return errors
}

// validateTraits validates the traits configuration
func (v *validationServiceImpl) validateTraits(traits apisv1.Traits, fieldPrefix string, isNested bool) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	// Validate storage traits
	for i, storage := range traits.Storage {
		errors = append(errors, v.validateStorageTrait(storage, fmt.Sprintf("%s.storage[%d]", fieldPrefix, i))...)
	}

	// Validate probe traits
	for i, probe := range traits.Probes {
		errors = append(errors, v.validateProbeTrait(probe, fmt.Sprintf("%s.probes[%d]", fieldPrefix, i))...)
	}

	// Validate init traits
	for i, init := range traits.Init {
		errors = append(errors, v.validateInitTrait(init, fmt.Sprintf("%s.init[%d]", fieldPrefix, i), isNested)...)
	}

	// Validate sidecar traits
	for i, sidecar := range traits.Sidecar {
		errors = append(errors, v.validateSidecarTrait(sidecar, fmt.Sprintf("%s.sidecar[%d]", fieldPrefix, i), isNested)...)
	}

	// Validate RBAC traits
	for i, rbac := range traits.RBAC {
		errors = append(errors, v.validateRBACTrait(rbac, fmt.Sprintf("%s.rbac[%d]", fieldPrefix, i))...)
	}

	// Validate Ingress traits
	for i, ingress := range traits.Ingress {
		errors = append(errors, v.validateIngressTrait(ingress, fmt.Sprintf("%s.ingress[%d]", fieldPrefix, i))...)
	}

	// Validate env_from traits
	for i, envFrom := range traits.EnvFrom {
		errors = append(errors, v.validateEnvFromTrait(envFrom, fmt.Sprintf("%s.env_from[%d]", fieldPrefix, i))...)
	}

	// Validate Envs traits
	for i, env := range traits.Envs {
		errors = append(errors, v.validateEnvsTrait(env, fmt.Sprintf("%s.envs[%d]", fieldPrefix, i))...)
	}

	// Validate Resources trait
	if traits.Resources != nil {
		errors = append(errors, v.validateResourcesTrait(*traits.Resources, fmt.Sprintf("%s.resources", fieldPrefix))...)
	}

	return errors
}

// validateStorageTrait validates a storage trait
func (v *validationServiceImpl) validateStorageTrait(storage spec.StorageTraitSpec, field string) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	// Validate type
	if storage.Type == "" {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.type", field),
			Code:    apisv1.ErrCodeMissingRequiredField,
			Message: "storage type is required",
		})
	} else if !validStorageTypes[storage.Type] {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.type", field),
			Code:    apisv1.ErrCodeInvalidStorageType,
			Message: fmt.Sprintf("invalid storage type: %s, must be one of: persistent, ephemeral, config, secret", storage.Type),
		})
	}

	// Validate mount_path
	if storage.MountPath == "" {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.mount_path", field),
			Code:    apisv1.ErrCodeMissingRequiredField,
			Message: "storage mount_path is required",
		})
	}

	// Validate size for persistent storage with tmpCreate=true
	if storage.Type == config.StorageTypePersistent && storage.TmpCreate && storage.Size != "" {
		if !storageQuantityRegexp.MatchString(storage.Size) {
			errors = append(errors, apisv1.ValidationError{
				Field:   fmt.Sprintf("%s.size", field),
				Code:    apisv1.ErrCodeInvalidStorageSize,
				Message: fmt.Sprintf("invalid storage size format: %s, must be a valid Kubernetes quantity (e.g., 1Gi, 500Mi)", storage.Size),
			})
		}
	}

	// Validate source_name for config/secret types
	if (storage.Type == config.StorageTypeConfig || storage.Type == config.StorageTypeSecret) && storage.SourceName == "" && storage.Name == "" {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.source_name", field),
			Code:    apisv1.ErrCodeMissingRequiredField,
			Message: "source_name or name is required for config/secret storage types",
		})
	}

	return errors
}

// validateProbeTrait validates a probe trait
func (v *validationServiceImpl) validateProbeTrait(probe spec.ProbeTraitsSpec, field string) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	// Validate type
	if probe.Type == "" {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.type", field),
			Code:    apisv1.ErrCodeMissingRequiredField,
			Message: "probe type is required",
		})
	} else if !validProbeTypes[probe.Type] {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.type", field),
			Code:    apisv1.ErrCodeInvalidProbeType,
			Message: fmt.Sprintf("invalid probe type: %s, must be one of: liveness, readiness, startup", probe.Type),
		})
	}

	// Validate that exactly one probe method is specified
	probeMethodCount := 0
	if probe.Exec != nil && len(probe.Exec.Command) > 0 {
		probeMethodCount++
	}
	if probe.HTTPGet != nil {
		probeMethodCount++
	}
	if probe.TCPSocket != nil {
		probeMethodCount++
	}

	if probeMethodCount == 0 {
		errors = append(errors, apisv1.ValidationError{
			Field:   field,
			Code:    apisv1.ErrCodeInvalidProbeConfig,
			Message: "probe must specify exactly one of exec, http_get, or tcp_socket",
		})
	} else if probeMethodCount > 1 {
		errors = append(errors, apisv1.ValidationError{
			Field:   field,
			Code:    apisv1.ErrCodeInvalidProbeConfig,
			Message: "probe must specify exactly one of exec, http_get, or tcp_socket, not multiple",
		})
	}

	// Validate HTTPGet probe
	if probe.HTTPGet != nil {
		if probe.HTTPGet.Port <= 0 {
			errors = append(errors, apisv1.ValidationError{
				Field:   fmt.Sprintf("%s.http_get.port", field),
				Code:    apisv1.ErrCodeMissingRequiredField,
				Message: "http_get probe port is required and must be positive",
			})
		}
	}

	// Validate TCPSocket probe
	if probe.TCPSocket != nil {
		if probe.TCPSocket.Port <= 0 {
			errors = append(errors, apisv1.ValidationError{
				Field:   fmt.Sprintf("%s.tcp_socket.port", field),
				Code:    apisv1.ErrCodeMissingRequiredField,
				Message: "tcp_socket probe port is required and must be positive",
			})
		}
	}

	return errors
}

// validateInitTrait validates an init container trait
func (v *validationServiceImpl) validateInitTrait(init spec.InitTraitSpec, field string, isNested bool) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	// Check for forbidden nested init
	if isNested {
		errors = append(errors, apisv1.ValidationError{
			Field:   field,
			Code:    apisv1.ErrCodeNestedTraitForbidden,
			Message: "init trait cannot be nested inside another init or sidecar trait",
		})
		return errors
	}

	// Validate image
	if init.Properties.Image == "" {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.properties.image", field),
			Code:    apisv1.ErrCodeMissingImage,
			Message: "init container image is required",
		})
	}

	// Validate nested traits (without init/sidecar)
	errors = append(errors, v.validateTraits(init.Traits, fmt.Sprintf("%s.traits", field), true)...)

	return errors
}

// validateSidecarTrait validates a sidecar container trait
func (v *validationServiceImpl) validateSidecarTrait(sidecar spec.SidecarTraitsSpec, field string, isNested bool) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	// Check for forbidden nested sidecar
	if isNested {
		errors = append(errors, apisv1.ValidationError{
			Field:   field,
			Code:    apisv1.ErrCodeNestedTraitForbidden,
			Message: "sidecar trait cannot be nested inside another init or sidecar trait",
		})
		return errors
	}

	// Validate image
	if sidecar.Image == "" {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.image", field),
			Code:    apisv1.ErrCodeMissingImage,
			Message: "sidecar container image is required",
		})
	}

	// Validate nested traits (without init/sidecar)
	errors = append(errors, v.validateTraits(sidecar.Traits, fmt.Sprintf("%s.traits", field), true)...)

	return errors
}

// validateRBACTrait validates an RBAC trait
func (v *validationServiceImpl) validateRBACTrait(rbac spec.RBACPolicySpec, field string) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	// Validate rules
	if len(rbac.Rules) == 0 {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.rules", field),
			Code:    apisv1.ErrCodeMissingRBACRules,
			Message: "rbac rules are required",
		})
		return errors
	}

	// Validate each rule has verbs
	for i, rule := range rbac.Rules {
		if len(rule.Verbs) == 0 {
			errors = append(errors, apisv1.ValidationError{
				Field:   fmt.Sprintf("%s.rules[%d].verbs", field, i),
				Code:    apisv1.ErrCodeMissingRBACVerbs,
				Message: "rbac rule verbs are required",
			})
		}
	}

	return errors
}

// validateIngressTrait validates an ingress trait
func (v *validationServiceImpl) validateIngressTrait(ingress spec.IngressTraitsSpec, field string) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	// Validate routes
	if len(ingress.Routes) == 0 {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.routes", field),
			Code:    apisv1.ErrCodeMissingIngressRoutes,
			Message: "ingress routes are required",
		})
		return errors
	}

	// Validate each route
	for i, route := range ingress.Routes {
		if route.Backend.ServiceName == "" {
			errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.routes[%d].backend.service_name", field, i),
			Code:    apisv1.ErrCodeMissingServiceName,
			Message: "ingress route backend service_name is required",
			})
		}
	}

	return errors
}

// validateEnvFromTrait validates an env_from trait
func (v *validationServiceImpl) validateEnvFromTrait(envFrom spec.EnvFromSourceSpec, field string) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	// Validate type
	if envFrom.Type == "" {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.type", field),
			Code:    apisv1.ErrCodeMissingRequiredField,
			Message: "env_from type is required",
		})
	} else if !validEnvFromTypes[envFrom.Type] {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.type", field),
			Code:    apisv1.ErrCodeInvalidEnvFromType,
			Message: fmt.Sprintf("invalid env_from type: %s, must be one of: secret, configMap", envFrom.Type),
		})
	}

	// Validate source_name
	if envFrom.SourceName == "" {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.source_name", field),
			Code:    apisv1.ErrCodeMissingRequiredField,
			Message: "env_from source_name is required",
		})
	}

	return errors
}

// validateEnvsTrait validates an envs trait
func (v *validationServiceImpl) validateEnvsTrait(env spec.SimplifiedEnvSpec, field string) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	// Validate name
	if env.Name == "" {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.name", field),
			Code:    apisv1.ErrCodeMissingRequiredField,
			Message: "env name is required",
		})
	}

	// Validate that exactly one value source is specified
	sourceCount := 0
	if env.ValueFrom.Static != nil {
		sourceCount++
	}
	if env.ValueFrom.Secret != nil {
		sourceCount++
	}
	if env.ValueFrom.Config != nil {
		sourceCount++
	}
	if env.ValueFrom.Field != nil {
		sourceCount++
	}

	if sourceCount == 0 {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.value_from", field),
			Code:    apisv1.ErrCodeInvalidEnvValueSource,
			Message: "env value_from must specify exactly one of static, secret, config, or field",
		})
	} else if sourceCount > 1 {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.value_from", field),
			Code:    apisv1.ErrCodeInvalidEnvValueSource,
			Message: "env value_from must specify exactly one of static, secret, config, or field, not multiple",
		})
	}

	// Validate secret reference
	if env.ValueFrom.Secret != nil {
		if env.ValueFrom.Secret.Name == "" {
			errors = append(errors, apisv1.ValidationError{
				Field:   fmt.Sprintf("%s.value_from.secret.name", field),
				Code:    apisv1.ErrCodeMissingRequiredField,
				Message: "secret name is required",
			})
		}
		if env.ValueFrom.Secret.Key == "" {
			errors = append(errors, apisv1.ValidationError{
				Field:   fmt.Sprintf("%s.value_from.secret.key", field),
				Code:    apisv1.ErrCodeMissingRequiredField,
				Message: "secret key is required",
			})
		}
	}

	// Validate config reference
	if env.ValueFrom.Config != nil {
		if env.ValueFrom.Config.Name == "" {
			errors = append(errors, apisv1.ValidationError{
				Field:   fmt.Sprintf("%s.value_from.config.name", field),
				Code:    apisv1.ErrCodeMissingRequiredField,
				Message: "configMap name is required",
			})
		}
		if env.ValueFrom.Config.Key == "" {
			errors = append(errors, apisv1.ValidationError{
				Field:   fmt.Sprintf("%s.value_from.config.key", field),
				Code:    apisv1.ErrCodeMissingRequiredField,
				Message: "configMap key is required",
			})
		}
	}

	return errors
}

// validateResourcesTrait validates a resources trait
func (v *validationServiceImpl) validateResourcesTrait(resources spec.ResourceTraitsSpec, field string) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	// Validate CPU format if provided
	if resources.CPU != "" && !isValidQuantity(resources.CPU) {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.cpu", field),
			Code:    apisv1.ErrCodeInvalidTraitConfig,
			Message: fmt.Sprintf("invalid CPU format: %s, must be a valid Kubernetes quantity (e.g., 500m, 1)", resources.CPU),
		})
	}

	// Validate memory format if provided
	if resources.Memory != "" && !isValidQuantity(resources.Memory) {
		errors = append(errors, apisv1.ValidationError{
			Field:   fmt.Sprintf("%s.memory", field),
			Code:    apisv1.ErrCodeInvalidTraitConfig,
			Message: fmt.Sprintf("invalid memory format: %s, must be a valid Kubernetes quantity (e.g., 512Mi, 1Gi)", resources.Memory),
		})
	}

	return errors
}

// isValidQuantity checks if a string is a valid Kubernetes resource quantity
func isValidQuantity(quantity string) bool {
	// Basic pattern for Kubernetes quantities
	quantityRegexp := regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(m|Ki|Mi|Gi|Ti|Pi|Ei|K|M|G|T|P|E)?$`)
	return quantityRegexp.MatchString(quantity)
}

// validateWorkflowSteps validates workflow steps and their component references
func (v *validationServiceImpl) validateWorkflowSteps(steps []apisv1.CreateWorkflowStepRequest, componentNames map[string]bool, fieldPrefix string) []apisv1.ValidationError {
	var errors []apisv1.ValidationError

	stepNames := make(map[string]bool)

	for i, step := range steps {
		stepField := fmt.Sprintf("%s[%d]", fieldPrefix, i)

		// Validate step name
		if step.Name != "" {
			nameErrors := v.validateName(step.Name, fmt.Sprintf("%s.name", stepField))
			errors = append(errors, nameErrors...)

			// Check for duplicate step names
			lowerName := strings.ToLower(step.Name)
			if stepNames[lowerName] {
				errors = append(errors, apisv1.ValidationError{
					Field:   fmt.Sprintf("%s.name", stepField),
					Code:    apisv1.ErrCodeDuplicateWorkflowStep,
					Message: fmt.Sprintf("duplicate workflow step name: %s", step.Name),
				})
			} else {
				stepNames[lowerName] = true
			}
		}

		// Validate mode
		if !validWorkflowModes[step.Mode] {
			errors = append(errors, apisv1.ValidationError{
				Field:   fmt.Sprintf("%s.mode", stepField),
				Code:    apisv1.ErrCodeInvalidWorkflowMode,
				Message: fmt.Sprintf("invalid workflow mode: %s, must be one of: StepByStep, DAG", step.Mode),
			})
		}

		// Collect all component references from step
		allComponents := mergeWorkflowComponents(step.Components, step.Properties.Policies)

		// Check if step has any components
		if len(allComponents) == 0 && len(step.SubSteps) == 0 {
			errors = append(errors, apisv1.ValidationError{
				Field:   stepField,
				Code:    apisv1.ErrCodeWorkflowStepNoComponent,
				Message: "workflow step must have at least one component or substep",
			})
		}

		// Validate component references
		for j, compName := range allComponents {
			if !componentNames[strings.ToLower(compName)] {
				errors = append(errors, apisv1.ValidationError{
					Field:   fmt.Sprintf("%s.components[%d]", stepField, j),
					Code:    apisv1.ErrCodeComponentNotFound,
					Message: fmt.Sprintf("component '%s' not found in application", compName),
				})
			}
		}

		// Validate substeps
		for j, subStep := range step.SubSteps {
			subStepField := fmt.Sprintf("%s.subSteps[%d]", stepField, j)

			// Validate substep name
			if subStep.Name != "" {
				errors = append(errors, v.validateName(subStep.Name, fmt.Sprintf("%s.name", subStepField))...)
			}

			// Collect all component references from substep
			subComponents := mergeWorkflowComponents(subStep.Components, subStep.Properties.Policies)

			// Validate component references in substep
			for k, compName := range subComponents {
				if !componentNames[strings.ToLower(compName)] {
					errors = append(errors, apisv1.ValidationError{
						Field:   fmt.Sprintf("%s.components[%d]", subStepField, k),
						Code:    apisv1.ErrCodeComponentNotFound,
						Message: fmt.Sprintf("component '%s' not found in application", compName),
					})
				}
			}
		}
	}

	return errors
}
