package spec

import corev1 "k8s.io/api/core/v1"

// NodeSelectionSpec describes node scheduling constraints.
type NodeSelectionSpec struct {
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Affinity     *corev1.Affinity    `json:"affinity,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
}
