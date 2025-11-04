package config

// ResourceKind identifies the category of Kubernetes resources managed by jobs.
type ResourceKind string

const (
	ResourceDeployment         ResourceKind = "deployment"
	ResourceStatefulSet        ResourceKind = "statefulset"
	ResourceService            ResourceKind = "service"
	ResourcePVC                ResourceKind = "pvc"
	ResourceConfigMap          ResourceKind = "configmap"
	ResourceSecret             ResourceKind = "secret"
	ResourceIngress            ResourceKind = "ingress"
	ResourceServiceAccount     ResourceKind = "serviceaccount"
	ResourceRole               ResourceKind = "role"
	ResourceRoleBinding        ResourceKind = "rolebinding"
	ResourceClusterRole        ResourceKind = "clusterrole"
	ResourceClusterRoleBinding ResourceKind = "clusterrolebinding"
)
