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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HealthCheckSpec defines the desired state of HealthCheck
type HealthCheckSpec struct {
	Enabled          bool                `json:"enabled,omitempty"`
	Invert           bool                `json:"invert,omitempty"`
	Protocol         HealthCheckProtocol `json:"protocol"`
	Port             int                 `json:"port"`
	Path             string              `json:"path,omitempty"`
	Endpoint         HealthCheckEndpoint `json:"endpoint,omitempty"`
	FailureThreshold int                 `json:"failureThreshold,omitempty"`
	Features         HealthCheckFeatures `json:"features"`
}

type HealthCheckFeatures struct {
	FastInterval bool   `json:"fastInterval,omitempty"`
	SearchString string `json:"searchString,omitempty"`
	LatencyGraph bool   `json:"latencyGraph,omitempty"`
}
type HealthCheckIPEndpoint struct {
	Address  string `json:"address,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

type HealthCheckProtocol string

var ProtocolHTTP HealthCheckProtocol = "HTTP"
var ProtocolHTTPS HealthCheckProtocol = "HTTPS"
var ProtocolTCP HealthCheckProtocol = "TCP"

// HealthCheckStatus defines the observed state of HealthCheck
type HealthCheckStatus struct {
	ID     string            `json:"id,omitempty"`
	Result HealthCheckResult `json:"result,omitempty"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

type HealthCheckResult string

var ResultHealthy HealthCheckResult = "Healthy"
var ResultUnhealthy HealthCheckResult = "Unhealthy"

// +kubebuilder:object:root=true

// HealthCheck is the Schema for the healthchecks API
type HealthCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HealthCheckSpec   `json:"spec,omitempty"`
	Status HealthCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HealthCheckList contains a list of HealthCheck
type HealthCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HealthCheck `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HealthCheck{}, &HealthCheckList{})
}
