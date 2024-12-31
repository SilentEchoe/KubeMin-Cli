package utils

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"context"
	"github.com/oam-dev/kubevela/pkg/features"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	if !features.APIServerFeatureGate.Enabled(features.APIServerEnableImpersonation) {
		return ctx
	}
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
	if ok && features.APIServerFeatureGate.Enabled(features.APIServerEnableAdminImpersonation) {
		for _, role := range userRole {
			if role == model.RoleAdmin {
				userInfo.Groups = []string{UXDefaultGroup}
			}
		}
	}
	return request.WithUser(ctx, userInfo)
}

func (c *AuthClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	// 根据上下文获取当前用户的信息
	ctx = ContextWithUserInfo(ctx)
	return c.Client.Get(ctx, key, obj)
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
	return &authAppStatusClient{StatusWriter: c.kubeClient.Status()}
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
