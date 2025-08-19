package spec

// This package defines canonical value objects shared by DTO and Domain.
// It avoids duplicating identical semantic structures across layers.

// Traits is the aggregate of all attachable traits for a component.
type Traits struct {
	Init      []InitTrait         `json:"init,omitempty"`
	Storage   []StorageTrait      `json:"storage,omitempty"`
	Secret    []SecretSpec        `json:"secret,omitempty"`
	Sidecar   []SidecarSpec       `json:"sidecar,omitempty"`
	EnvFrom   []EnvFromSourceSpec `json:"envFrom,omitempty"`
	Envs      []SimplifiedEnvSpec `json:"envs,omitempty"`
	Probes    []ProbeSpec         `json:"probes,omitempty"`
	Resources *ResourceSpec       `json:"resources,omitempty"`
}

// InitTrait describes an init container with its own nested traits.
type InitTrait struct {
	Name       string     `json:"name"`
	Traits     Traits     `json:"traits,omitempty"`
	Properties Properties `json:"properties"`
}

// StorageTrait describes storage characteristics for mounting into containers.
type StorageTrait struct {
	Name       string `json:"name,omitempty"`
	Type       string `json:"type"`
	MountPath  string `json:"mountPath"`
	SubPath    string `json:"subPath,omitempty"`
	ReadOnly   bool   `json:"readOnly,omitempty"`
	SourceName string `json:"sourceName,omitempty"` // For ConfigMap/Secret volume sources

	// For "persistent" type
	Create    bool   `json:"create,omitempty"`    // If true, create PVC. Defaults to false (referencing existing).
	Size      string `json:"size,omitempty"`      // Used when Create is true.
	ClaimName string `json:"claimName,omitempty"` // Name of existing PVC to use. If empty, defaults to Name.
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

// ProbeSpec defines a health check probe for a container.
type ProbeSpec struct {
	Type                string          `json:"type"` // "liveness", "readiness", or "startup"
	InitialDelaySeconds int32           `json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int32           `json:"periodSeconds,omitempty"`
	TimeoutSeconds      int32           `json:"timeoutSeconds,omitempty"`
	FailureThreshold    int32           `json:"failureThreshold,omitempty"`
	SuccessThreshold    int32           `json:"successThreshold,omitempty"`
	Exec                *ExecProbe      `json:"exec,omitempty"`
	HTTPGet             *HTTPGetProbe   `json:"httpGet,omitempty"`
	TCPSocket           *TCPSocketProbe `json:"tcpSocket,omitempty"`
}

// ExecProbe describes a command-line probe.
type ExecProbe struct {
	Command []string `json:"command"`
}

// HTTPGetProbe describes an HTTP probe.
type HTTPGetProbe struct {
	Path   string `json:"path"`
	Port   int    `json:"port"`
	Host   string `json:"host,omitempty"`
	Scheme string `json:"scheme,omitempty"`
}

// TCPSocketProbe describes a TCP socket probe.
type TCPSocketProbe struct {
	Port int    `json:"port"`
	Host string `json:"host,omitempty"`
}

// ResourceSpec defines CPU/Memory/GPU resources for a container.
// It is modeled as a trait so it can be attached to main, init, or sidecar containers (via nested traits).
// Values should be valid Kubernetes quantities, e.g., "500m" for CPU and "256Mi" for memory.
type ResourceSpec struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
	GPU    string `json:"gpu,omitempty"`
}
