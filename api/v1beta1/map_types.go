/*

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PhasePending = "PENDING"
	PhaseRunning = "RUNNING"
	PhaseFailed  = "FAILED"
)

// MapSpec defines the desired state of Map
type MapSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// TCP/UDP protocol
	Protocol string `json:"protocol,omitempty"`

	// Port
	Port int `json:"port,omitempty"`

	// Host
	Host string `json:"host,omitempty"`

	// LivenessProbe
	LivenessProbe bool `json:"liveness_probe"`
}

// MapStatus defines the observed state of Map
type MapStatus struct {
	Phase string `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true

// Map is the Schema for the maps API
type Map struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MapSpec   `json:"spec,omitempty"`
	Status MapStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MapList contains a list of Map
type MapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Map `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Map{}, &MapList{})
}
