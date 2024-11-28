package apply

import (
	"KubeMin-Cli/pkg/apiserver/utils"
	"k8s.io/client-go/kubernetes"
)

// APIApplicator implements Applicator
type APIApplicator struct {
	c *kubernetes.Clientset
}

// NewAPIApplicator creates an Applicator that applies state to an
// object or creates the object if not exist.
func NewAPIApplicator(c *utils.AuthClient) *APIApplicator {
	return &APIApplicator{
		c: c.Clientset,
	}
}
