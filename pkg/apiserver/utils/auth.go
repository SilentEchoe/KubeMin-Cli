package utils

import (
	"context"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AuthClient struct {
	*kubernetes.Clientset
}

func (c *AuthClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *AuthClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *AuthClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *AuthClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *AuthClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *AuthClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	//TODO implement me
	panic("implement me")
}

func (c *AuthClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	//TODO implement me
	panic("implement me")
}

type authAppStatusClient struct {
	client.StatusWriter
}

func (c *AuthClient) Status() client.SubResourceWriter {
	return &authAppStatusClient{StatusWriter: c.Client.Status()}
}

func (c *AuthClient) SubResource(subResource string) client.SubResourceClient {
	//TODO implement me
	panic("implement me")
}

func (c *AuthClient) Scheme() *runtime.Scheme {
	//TODO implement me
	panic("implement me")
}

func (c *AuthClient) RESTMapper() meta.RESTMapper {
	//TODO implement me
	panic("implement me")
}

func (c *AuthClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	//TODO implement me
	panic("implement me")
}

func (c *AuthClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	//TODO implement me
	panic("implement me")
}

// NewAuthClient will carry UserInfo for mutating requests automatically
func NewAuthClient(c *kubernetes.Clientset) client.Client {
	return &AuthClient{}
}
