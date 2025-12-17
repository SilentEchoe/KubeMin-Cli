package job

import (
	"KubeMin-Cli/pkg/apiserver/workflow/naming"
)

func buildWebServiceName(name, appID string) string { return naming.WebServiceName(name, appID) }
func buildServiceName(name, appID string) string    { return naming.ServiceName(name, appID) }
func buildStoreSeverName(name, appID string) string { return naming.StoreServerName(name, appID) }
func buildSharedWebServiceName(name string) string  { return naming.SharedWebServiceName(name) }
func buildSharedServiceName(name string) string     { return naming.SharedServiceName(name) }
func buildSharedStoreServerName(name string) string { return naming.SharedStoreServerName(name) }

// BuildIngressName returns a normalized ingress resource name for the given component/app.
func BuildIngressName(name, appID string) string { return naming.IngressName(name, appID) }

// BuildSharedIngressName returns a normalized ingress resource name without app scoping.
func BuildSharedIngressName(name string) string { return naming.SharedIngressName(name) }
