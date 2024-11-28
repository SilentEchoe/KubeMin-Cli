package utils

import "k8s.io/client-go/kubernetes"

type AuthClient struct {
	*kubernetes.Clientset
}

// NewAuthClient will carry UserInfo for mutating requests automatically

func NewAuthClient(c *kubernetes.Clientset) *AuthClient {
	return &AuthClient{c}
}
