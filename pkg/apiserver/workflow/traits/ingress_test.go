package traits

import (
	"testing"

	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"

	"kubemin-cli/pkg/apiserver/domain/model"
	spec "kubemin-cli/pkg/apiserver/domain/spec"
)

func TestBuildIngress_WithAnnotationsAndClass(t *testing.T) {
	trait := &spec.IngressTraitsSpec{
		Name:             "catanddog-2510301134udp0bg-frontend",
		Namespace:        "2505131620u7b9hq",
		IngressClassName: "ingress-nginx",
		Label: map[string]string{
			"frontPurchaserProductId": "31",
			"name":                    "catanddog-2510301134udp0bg-frontend",
		},
		Annotations: map[string]string{
			"nginx.ingress.kubernetes.io/proxy-read-timeout": "60",
			"nginx.ingress.kubernetes.io/proxy-send-timeout": "60",
			"nginx.ingress.kubernetes.io/rewrite-target":     "$1",
			"nginx.ingress.kubernetes.io/use-regex":          "true",
		},
		Routes: []spec.IngressRoutes{
			{
				Path:     "/27d51e4eae211962f00b63622d0274b0(/.*)",
				PathType: "ImplementationSpecific",
				Host:     "3os-game.test.yu3.co",
				Backend: spec.IngressRoute{
					ServiceName: "catanddog-2510301134udp0bg-frontend",
					ServicePort: 80,
				},
			},
		},
	}

	ing, err := BuildIngress(trait)
	require.NoError(t, err)

	require.Equal(t, trait.Name, ing.Name)
	require.Equal(t, trait.Namespace, ing.Namespace)
	require.Equal(t, trait.Label["frontPurchaserProductId"], ing.Labels["frontPurchaserProductId"])
	require.Equal(t, trait.Annotations["nginx.ingress.kubernetes.io/rewrite-target"], ing.Annotations["nginx.ingress.kubernetes.io/rewrite-target"])
	require.NotNil(t, ing.Spec.IngressClassName)
	require.Equal(t, trait.IngressClassName, *ing.Spec.IngressClassName)

	require.Len(t, ing.Spec.Rules, 1)
	rule := ing.Spec.Rules[0]
	require.Equal(t, trait.Routes[0].Host, rule.Host)
	require.NotNil(t, rule.HTTP)
	require.Len(t, rule.HTTP.Paths, 1)
	ingPath := rule.HTTP.Paths[0]
	require.Equal(t, trait.Routes[0].Path, ingPath.Path)
	require.NotNil(t, ingPath.PathType)
	require.Equal(t, networkingv1.PathTypeImplementationSpecific, *ingPath.PathType)
	require.NotNil(t, ingPath.Backend.Service)
	require.Equal(t, trait.Routes[0].Backend.ServiceName, ingPath.Backend.Service.Name)
	require.Equal(t, trait.Routes[0].Backend.ServicePort, ingPath.Backend.Service.Port.Number)
}

func TestDeterminePathType_Defaults(t *testing.T) {
	spec := &model.IngressTraitsSpec{
		DefaultPathType: "Exact",
	}
	route := model.IngressRoutes{}
	pt := determinePathType(route, spec, nil)
	require.Equal(t, networkingv1.PathTypeExact, pt)

	route.PathType = "ImplementationSpecific"
	pt = determinePathType(route, spec, map[string]string{})
	require.Equal(t, networkingv1.PathTypeImplementationSpecific, pt)
}

func TestApplyIngressDefaultsNamespaceUsesComponent(t *testing.T) {
	component := &model.ApplicationComponent{Namespace: "component-ns"}
	trait := &spec.IngressTraitsSpec{Namespace: "custom-ns"}

	applyIngressDefaults(trait, component, 0)
	require.Equal(t, "component-ns", trait.Namespace)
}
