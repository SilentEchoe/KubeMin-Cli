package job

import (
	"KubeMin-Cli/pkg/apiserver/utils/naming"
	wfNaming "KubeMin-Cli/pkg/apiserver/workflow/naming"
)

func buildWebServiceName(name, appID string) string { return naming.WebServiceName(name, appID) }
func buildServiceName(name, appID string) string    { return naming.ServiceName(name, appID) }
func buildIngressName(name, appID string) string    { return naming.IngressName(name, appID) }
func buildPVCName(name, appID string) string        { return wfNaming.PVCName(name, appID) }
func buildConfigMapName(name, appID string) string  { return naming.ConfigMapName(name, appID) }
func buildSecretName(name, appID string) string     { return naming.SecretName(name, appID) }
func buildStoreSeverName(name, appID string) string { return naming.StoreServerName(name, appID) }

// BuildIngressName returns a normalized ingress resource name for the given component/app.
func BuildIngressName(name, appID string) string { return naming.IngressName(name, appID) }

// BuildPVCName returns a normalized PVC resource name for the given component/app.
func BuildPVCName(name, appID string) string { return wfNaming.PVCName(name, appID) }

// BuildConfigMapName returns a normalized ConfigMap resource name for the given component/app.
func BuildConfigMapName(name, appID string) string { return naming.ConfigMapName(name, appID) }

// BuildSecretName returns a normalized Secret resource name for the given component/app.
func BuildSecretName(name, appID string) string { return naming.SecretName(name, appID) }

// BuildServiceName returns a normalized Service resource name for the given component/app.
func BuildServiceName(name, appID string) string { return naming.ServiceName(name, appID) }

// BuildStoreServerName returns a normalized store workload name for the given component/app.
func BuildStoreServerName(name, appID string) string { return naming.StoreServerName(name, appID) }
