/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationsSpec defines the desired state of Applications.
type ApplicationsSpec struct {
	// 组件
}

// ApplicationsStatus defines the observed state of Applications.
type ApplicationsStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Applications is the Schema for the applications API.
type Applications struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationsSpec   `json:"spec,omitempty"`
	Status ApplicationsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationsList contains a list of Applications.
type ApplicationsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Applications `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Applications{}, &ApplicationsList{})
}
