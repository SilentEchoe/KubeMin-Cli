package traits

import (
	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/model"
	
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	spec "KubeMin-Cli/pkg/apiserver/domain/spec"
	networkingv1 "k8s.io/api/networking/v1"
)

type IngressProcessor struct{}

func (p *IngressProcessor) Name() string { return "ingress" }

func (p *IngressProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	traits, ok := ctx.TraitData.([]spec.IngressTraitsSpec)
	if !ok {
		return nil, fmt.Errorf("ingress trait expects []spec.IngressTraitSpec, got %T", ctx.TraitData)
	}
	if len(traits) == 0 {
		return nil, nil
	}

	result := &TraitResult{}
	for idx, t := range traits {
		// Apply defaults
		applyIngressDefaults(&t, ctx.Component, idx)

		// Build ingress resource
		obj, err := BuildIngress(&t)
		if err != nil {
			return nil, fmt.Errorf("build ingress trait[%d]: %w", idx, err)
		}
		result.AdditionalObjects = append(result.AdditionalObjects, obj)
	}
	return result, nil
}

func applyIngressDefaults(traitSpec *spec.IngressTraitsSpec, component *model.ApplicationComponent, idx int) {
	if traitSpec == nil {
		return
	}
	if traitSpec.Name == "" && component != nil {
		base := strings.ToLower(component.Name)
		suffix := "ingress"
		if idx > 0 {
			suffix = fmt.Sprintf("ingress-%d", idx+1)
		}
		traitSpec.Name = fmt.Sprintf("%s-%s", base, suffix)
	}

	if traitSpec.Namespace == "" && component != nil {
		traitSpec.Namespace = component.Namespace
	} else {
		traitSpec.Namespace = config.DefaultNamespace
	}
}

func BuildIngress(ingressSpec *spec.IngressTraitsSpec) (*networkingv1.Ingress, error) {
	if ingressSpec == nil {
		return nil, fmt.Errorf("ingress spec is nil")
	}

	if len(ingressSpec.Routes) == 0 {
		return nil, fmt.Errorf("at least one route is required")
	}

	return buildIngressFromSpec(&model.IngressTraitsSpec{
		Name:      ingressSpec.Name,
		Namespace: ingressSpec.Namespace,
		TLS:       ingressSpec.TLS,
		Routes:    ingressSpec.Routes,
	}), nil
}

func buildIngressFromSpec(ingressSpec *spec.IngressTraitsSpec) *networkingv1.Ingress {
	ingress := networkingv1.IngressSpec{
		TLS: convertTLS(ingressSpec.TLS),
	}

	hostRules := map[string]*networkingv1.HTTPIngressRuleValue{}
	var hostOrder []string

	for _, route := range ingressSpec.Routes {
		path := route.Path
		if path == "" {
			path = "/"
		}
		pathType := networkingv1.PathTypePrefix
		backend := convertBackend(route.Backend)

		targetHosts := deriveHosts(ingressSpec, route)
		for _, host := range targetHosts {
			if _, ok := hostRules[host]; !ok {
				hostRules[host] = &networkingv1.HTTPIngressRuleValue{}
				hostOrder = append(hostOrder, host)
			}
			hostRules[host].Paths = append(hostRules[host].Paths, networkingv1.HTTPIngressPath{
				Path:     path,
				PathType: &pathType,
				Backend:  backend,
			})
		}

		for _, host := range hostOrder {
			rule := networkingv1.IngressRule{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: hostRules[host],
				},
			}
			if host != "" {
				rule.Host = host
			}
			ingress.Rules = append(ingress.Rules, rule)
		}

	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressSpec.Name,
			Namespace: ingressSpec.Namespace,
			Labels:    ingressSpec.Label,
		},
		Spec: ingress,
	}
	ing.SetGroupVersionKind(networkingv1.SchemeGroupVersion.WithKind("Ingress"))
	return ing
}

func convertTLS(src []model.IngressTLSConfig) []networkingv1.IngressTLS {
	if len(src) == 0 {
		return nil
	}
	res := make([]networkingv1.IngressTLS, 0, len(src))
	for _, tls := range src {
		hosts := append([]string(nil), tls.Hosts...)
		res = append(res, networkingv1.IngressTLS{
			Hosts:      hosts,
			SecretName: tls.SecretName,
		})
	}
	return res
}

func convertBackend(route model.IngressRoute) networkingv1.IngressBackend {
	port := route.ServicePort
	if port <= 0 {
		port = 80
	}
	return networkingv1.IngressBackend{
		Service: &networkingv1.IngressServiceBackend{
			Name: route.ServiceName,
			Port: networkingv1.ServiceBackendPort{
				Number: port,
			},
		},
	}
}

func deriveHosts(feature *model.IngressTraitsSpec, route spec.IngressRoutes) []string {
	if route.Host != "" {
		return []string{route.Host}
	}
	if len(feature.Hosts) > 0 {
		return append([]string(nil), feature.Hosts...)
	}
	return []string{""}
}
