package job

import (
	"KubeMin-Cli/pkg/apiserver/workflow/naming"
)

func buildWebServiceName(name, appID string) string { return naming.WebServiceName(name, appID) }
func buildServiceName(name, appID string) string    { return naming.ServiceName(name, appID) }
func buildStoreSeverName(name, appID string) string { return naming.StoreServerName(name, appID) }

// BuildIngressName returns a normalized ingress resource name for the given component/app.
func BuildIngressName(name, appID string) string { return naming.IngressName(name, appID) }
