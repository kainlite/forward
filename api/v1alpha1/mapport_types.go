/*
Copyright 2024.

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

const (
	PhasePending = "PENDING"
	PhaseRunning = "RUNNING"
	PhaseFailed  = "FAILED"
)

// MapPortSpec defines the desired state of MapPort.
type MapPortSpec struct {
	// TCP/UDP protocol
	Protocol string `json:"protocol,omitempty"`

	// Port
	Port int `json:"port,omitempty"`

	// Host
	Host string `json:"host,omitempty"`
}

// MapPortStatus defines the observed state of MapPort.
type MapPortStatus struct {
	Phase string `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MapPort is the Schema for the mapports API.
type MapPort struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MapPortSpec   `json:"spec,omitempty"`
	Status MapPortStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MapPortList contains a list of MapPort.
type MapPortList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MapPort `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MapPort{}, &MapPortList{})
}
