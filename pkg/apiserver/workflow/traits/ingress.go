package traits

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/event/workflow/job"
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
	}
	traitSpec.Label = job.BuildLabels(component, nil)
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
		Default:   ingressSpec.Default,
		Routes:    ingressSpec.Routes,
	}), nil
}

func buildIngressFromSpec(ingressSpec *spec.IngressTraitsSpec) *networkingv1.Ingress {
	ingress := networkingv1.IngressSpec{
		TLS: convertTLS(ingressSpec.TLS),
	}

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressSpec.Name,
			Namespace: ingressSpec.Namespace,
			Labels:    ingressSpec.Label,
		},
		Spec: ingress,
	}
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
