package spec

// This package defines canonical value objects shared by DTO and Domain.
// It avoids duplicating identical semantic structures across layers.

// Traits is the aggregate of all attachable traits for a component.
type Traits struct {
	Init    []InitTrait         `json:"init,omitempty"`
	Storage []StorageTrait      `json:"storage,omitempty"`
	Secret  []SecretSpec        `json:"secret,omitempty"`
	Sidecar []SidecarSpec       `json:"sidecar,omitempty"`
	EnvFrom []EnvFromSourceSpec `json:"envFrom,omitempty"`
	Envs    []SimplifiedEnvSpec `json:"envs,omitempty"`
}

// InitTrait describes an init container with its own nested traits.
type InitTrait struct {
	Name       string     `json:"name"`
	Traits     []Traits   `json:"traits"`
	Properties Properties `json:"properties"`
}

// StorageTrait describes storage characteristics for mounting into containers.
type StorageTrait struct {
	Name      string `json:"name,omitempty"`
	Type      string `json:"type"`
	MountPath string `json:"mountPath"`
	Size      string `json:"size"`
	SubPath   string `json:"subPath"`
	ReadOnly  bool   `json:"readOnly"`
	// For ConfigMap/Secret volume sources
	SourceName string `json:"sourceName,omitempty"`
}

type ConfigMapSpec struct {
	Data map[string]string `json:"data"`
}

type SecretSpec struct {
	Data map[string]string `json:"data"`
}

// SidecarSpec describes a sidecar container that may attach additional traits.
type SidecarSpec struct {
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Command []string          `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Traits  Traits            `json:"traits,omitempty"`
}

// EnvFromSourceSpec corresponds to a single corev1.EnvFromSource.
type EnvFromSourceSpec struct {
	Type       string `json:"type"`       // "secret" or "configMap"
	SourceName string `json:"sourceName"` // The name of the secret or configMap
}

// SimplifiedEnvSpec is the user-friendly, simplified way to define environment variables.
type SimplifiedEnvSpec struct {
	Name      string      `json:"name"`
	ValueFrom ValueSource `json:"valueFrom"`
}

// ValueSource defines the source for an environment variable's value.
// Only one of its fields may be set.
type ValueSource struct {
	Static *string                `json:"static,omitempty"`
	Secret *SecretSelectorSpec    `json:"secret,omitempty"`
	Config *ConfigMapSelectorSpec `json:"config,omitempty"`
	Field  *string                `json:"field,omitempty"`
}

// SecretSelectorSpec selects a key from a Secret.
type SecretSelectorSpec struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// ConfigMapSelectorSpec selects a key from a ConfigMap.
type ConfigMapSelectorSpec struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// Properties describes container-level properties shared by traits.
type Properties struct {
	Image   string            `json:"image"`
	Ports   []Ports           `json:"ports"`
	Env     map[string]string `json:"env"`
	Command []string          `json:"command"`
	Labels  map[string]string `json:"labels"`
}

type Ports struct {
	Port   int32 `json:"port"`
	Expose bool  `json:"expose"`
}
