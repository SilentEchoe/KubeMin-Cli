package traits

import (
	spec "KubeMin-Cli/pkg/apiserver/spec"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	probeTypeLiveness  = "liveness"
	probeTypeReadiness = "readiness"
	probeTypeStartup   = "startup"
)

// ProbeProcessor attaches container health checks (liveness/readiness/startup).
type ProbeProcessor struct{}

// Name returns the name of the trait.
func (p *ProbeProcessor) Name() string {
	return "probes"
}

// Process converts []spec.ProbeSpec into Kubernetes Probe objects. Only one
// probe per type is allowed; duplicates result in an error.
func (p *ProbeProcessor) Process(ctx *TraitContext) (*TraitResult, error) {
	probeTraits, ok := ctx.TraitData.([]spec.ProbeSpec)
	if !ok {
		return nil, fmt.Errorf("unexpected type for probes trait: %T", ctx.TraitData)
	}

	result := &TraitResult{}

	for _, probeSpec := range probeTraits {
		kubeProbe, err := p.convertSpecToKubeProbe(probeSpec)
		if err != nil {
			return nil, err
		}

		switch probeSpec.Type {
		case probeTypeLiveness:
			if result.LivenessProbe != nil {
				return nil, fmt.Errorf("liveness probe already defined for component %s", ctx.Component.Name)
			}
			result.LivenessProbe = kubeProbe
		case probeTypeReadiness:
			if result.ReadinessProbe != nil {
				return nil, fmt.Errorf("readiness probe already defined for component %s", ctx.Component.Name)
			}
			result.ReadinessProbe = kubeProbe
		case probeTypeStartup:
			if result.StartupProbe != nil {
				return nil, fmt.Errorf("startup probe already defined for component %s", ctx.Component.Name)
			}
			result.StartupProbe = kubeProbe
		default:
			return nil, fmt.Errorf("invalid probe type: %s. Must be one of '%s', '%s', or '%s'",
				probeSpec.Type, probeTypeLiveness, probeTypeReadiness, probeTypeStartup)
		}
	}

	return result, nil
}

// convertSpecToKubeProbe converts a simplified spec into a Kubernetes Probe object.
func (p *ProbeProcessor) convertSpecToKubeProbe(spec spec.ProbeSpec) (*corev1.Probe, error) {
	probe := &corev1.Probe{
		InitialDelaySeconds: spec.InitialDelaySeconds,
		PeriodSeconds:       spec.PeriodSeconds,
		TimeoutSeconds:      spec.TimeoutSeconds,
		FailureThreshold:    spec.FailureThreshold,
		SuccessThreshold:    spec.SuccessThreshold,
	}

	// Ensure exactly one probe handler is specified
	handlerCount := 0
	if spec.Exec != nil {
		handlerCount++
	}
	if spec.HTTPGet != nil {
		handlerCount++
	}
	if spec.TCPSocket != nil {
		handlerCount++
	}
	if handlerCount != 1 {
		return nil, fmt.Errorf("exactly one of 'exec', 'httpGet', or 'tcpSocket' must be specified for a probe")
	}

	if spec.Exec != nil {
		probe.ProbeHandler.Exec = &corev1.ExecAction{Command: spec.Exec.Command}
	}
	if spec.HTTPGet != nil {
		probe.ProbeHandler.HTTPGet = &corev1.HTTPGetAction{
			Path:   spec.HTTPGet.Path,
			Port:   intstr.FromInt(spec.HTTPGet.Port),
			Host:   spec.HTTPGet.Host,
			Scheme: corev1.URIScheme(spec.HTTPGet.Scheme),
		}
	}
	if spec.TCPSocket != nil {
		probe.ProbeHandler.TCPSocket = &corev1.TCPSocketAction{
			Port: intstr.FromInt(spec.TCPSocket.Port),
			Host: spec.TCPSocket.Host,
		}
	}

	return probe, nil
}
