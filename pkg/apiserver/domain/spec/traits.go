package spec

// This package defines canonical value objects shared by DTO and Domain.
// It avoids duplicating identical semantic structures across layers.

// Traits is the aggregate of all attachable traits for a component.
type Traits struct {
	Init      []InitTraitSpec     `json:"init,omitempty"`
	Storage   []StorageTraitSpec  `json:"storage,omitempty"`
	Sidecar   []SidecarTraitsSpec `json:"sidecar,omitempty"`
	Ingress   []IngressTraitsSpec `json:"ingress,omitempty"`
	RBAC      []RBACPolicySpec    `json:"rbac,omitempty"`
	EnvFrom   []EnvFromSourceSpec `json:"envFrom,omitempty"`
	Envs      []SimplifiedEnvSpec `json:"envs,omitempty"`
	Probes    []ProbeTraitsSpec   `json:"probes,omitempty"`
	Resources *ResourceTraitsSpec `json:"resources,omitempty"`
}

// InitTraitSpec describes an init container with its own nested traits.
type InitTraitSpec struct {
	Name       string     `json:"name"`
	Traits     Traits     `json:"traits,omitempty"`
	Properties Properties `json:"properties"`
}

// StorageTraitSpec describes storage characteristics for mounting into containers.
type StorageTraitSpec struct {
	Name       string `json:"name,omitempty"`
	Type       string `json:"type"`
	MountPath  string `json:"mountPath"`
	SubPath    string `json:"subPath,omitempty"`
	ReadOnly   bool   `json:"readOnly,omitempty"`
	SourceName string `json:"sourceName,omitempty"` // For ConfigMap/Secret volume sources

	// For "persistent" type
	Create       bool   `json:"create,omitempty"`       // If true, create PVC. Defaults to false (referencing existing).
	Size         string `json:"size,omitempty"`         // Used when Create is true.
	ClaimName    string `json:"claimName,omitempty"`    // Name of existing PVC to use. If empty, defaults to Name.
	StorageClass string `json:"storageClass,omitempty"` // StorageClass to use for the PVC.
}

type ConfigMapSpec struct {
	Data map[string]string `json:"data"`
}

type SecretTraitsSpec struct {
	Data map[string]string `json:"data"`
}

// SidecarTraitsSpec describes a sidecar container that may attach additional traits.
type SidecarTraitsSpec struct {
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
	Conf    map[string]string `json:"conf"`
	Secret  map[string]string `json:"secret"`
	Command []string          `json:"command"`
	Labels  map[string]string `json:"labels"`
}

type Ports struct {
	Port   int32 `json:"port"`
	Expose bool  `json:"expose"`
}

// ProbeTraitsSpec defines a health check probe for a container.
type ProbeTraitsSpec struct {
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

// ResourceTraitsSpec defines CPU/Memory/GPU resources for a container.
// It is modeled as a trait so it can be attached to main, init, or sidecar containers (via nested traits).
// Values should be valid Kubernetes quantities, e.g., "500m" for CPU and "256Mi" for memory.
type ResourceTraitsSpec struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
	GPU    string `json:"gpu,omitempty"`
}

// IngressTraitsSpec captures the high-level ingress description.
// All configuration is done through the unified Routes field.
type IngressTraitsSpec struct {
	Name             string             `json:"name"`
	Namespace        string             `json:"namespace"`
	Hosts            []string           `json:"hosts,omitempty"`
	Label            map[string]string  `json:"label"`
	Annotations      map[string]string  `json:"annotations,omitempty"`
	IngressClassName string             `json:"ingressClassName,omitempty"`
	DefaultPathType  string             `json:"defaultPathType,omitempty"`
	TLS              []IngressTLSConfig `json:"tls,omitempty"`
	Routes           []IngressRoutes    `json:"routes"`
}
type IngressTLSConfig struct {
	SecretName string   `json:"secretName"`
	Hosts      []string `json:"hosts,omitempty"`
}

type IngressRoutes struct {
	Path     string       `json:"path,omitempty"`
	PathType string       `json:"pathType,omitempty"`
	Host     string       `json:"host,omitempty"`
	Backend  IngressRoute `json:"backend"`
	// Route-level optional features
	Rewrite *RewritePolicy `json:"rewrite,omitempty"`
}

type IngressRoute struct {
	ServiceName string            `json:"serviceName"`
	ServicePort int32             `json:"servicePort,omitempty"`
	Weight      int32             `json:"weight,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

type RewritePolicy struct {
	Type        string `json:"type"` // e.g. "replace", "regexReplace", "prefix"
	Match       string `json:"match,omitempty"`
	Replacement string `json:"replacement,omitempty"`
}

// RBACPolicySpec describes an RBAC policy to be created for the component.
type RBACPolicySpec struct {
	ServiceAccount             string            `json:"serviceAccount,omitempty"`
	Namespace                  string            `json:"namespace,omitempty"`
	ClusterScope               bool              `json:"clusterScope,omitempty"`
	RoleName                   string            `json:"roleName,omitempty"`
	BindingName                string            `json:"bindingName,omitempty"`
	ServiceAccountLabels       map[string]string `json:"serviceAccountLabels,omitempty"`
	ServiceAccountAnnotations  map[string]string `json:"serviceAccountAnnotations,omitempty"`
	RoleLabels                 map[string]string `json:"roleLabels,omitempty"`
	BindingLabels              map[string]string `json:"bindingLabels,omitempty"`
	Rules                      []RBACRuleSpec    `json:"rules"`
	ServiceAccountAutomountSAT *bool             `json:"automountServiceAccountToken,omitempty"`
}

// RBACRuleSpec mirrors rbacv1.PolicyRule with common fields exposed.
type RBACRuleSpec struct {
	APIGroups       []string `json:"apiGroups,omitempty"`
	Resources       []string `json:"resources,omitempty"`
	ResourceNames   []string `json:"resourceNames,omitempty"`
	NonResourceURLs []string `json:"nonResourceURLs,omitempty"`
	Verbs           []string `json:"verbs"`
}
