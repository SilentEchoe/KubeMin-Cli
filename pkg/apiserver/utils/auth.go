package utils

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"context"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KubeMinCliProjectGroupPrefix the prefix kubeMinCli project
const KubeMinCliProjectGroupPrefix = "kubemincli:project:"
const KubeMinCliClientGroup = "kubemincli:client"

// UXDefaultGroup This group means directly using the original identity registered by the cluster.
const UXDefaultGroup = "kubemincli:ux"

type AuthClient struct {
	client.Client
}

func ContextWithUserInfo(ctx context.Context) context.Context {
	userInfo := &user.DefaultInfo{Name: user.Anonymous}
	if username, ok := UsernameFrom(ctx); ok {
		userInfo.Name = username
	}
	if project, ok := ProjectFrom(ctx); ok && project != "" {
		userInfo.Groups = []string{KubeMinCliProjectGroupPrefix + project, KubeMinCliClientGroup}
	} else {
		userInfo.Groups = []string{UXDefaultGroup}
	}
	userRole, ok := UserRoleFrom(ctx)

	//You can add an environment variable to determine whether dev mode is enabled
	if ok {
		for _, role := range userRole {
			if role == model.RoleAdmin {
				userInfo.Groups = []string{UXDefaultGroup}
			}
		}
	}
	return request.WithUser(ctx, userInfo)
}

// NewAuthClient will carry UserInfo for mutating requests automatically
func NewAuthClient(c *kubernetes.Clientset) client.Client {
	return &AuthClient{}
}

func (c *AuthClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	ctx = ContextWithUserInfo(ctx)
	return c.Client.Get(ctx, key, obj)
}

func (c *AuthClient) List(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
	ctx = ContextWithUserInfo(ctx)
	return c.Client.List(ctx, obj, opts...)
}

func (c *AuthClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	ctx = ContextWithUserInfo(ctx)
	return c.Client.Create(ctx, obj, opts...)
}

func (c *AuthClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	ctx = ContextWithUserInfo(ctx)
	return c.Client.Delete(ctx, obj, opts...)
}

func (c *AuthClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	ctx = ContextWithUserInfo(ctx)
	return c.Client.Update(ctx, obj, opts...)
}

func (c *AuthClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	ctx = ContextWithUserInfo(ctx)
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func (c *AuthClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	ctx = ContextWithUserInfo(ctx)
	return c.Client.DeleteAllOf(ctx, obj, opts...)
}

type authAppStatusClient struct {
	client.StatusWriter
}

func (c *AuthClient) Status() client.SubResourceWriter {
	return &authAppStatusClient{StatusWriter: c.Client.Status()}
}
