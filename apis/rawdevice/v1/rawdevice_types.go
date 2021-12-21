/*
Copyright 2021.

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

// RawDeviceSpec defines the desired state of RawDevice
type RawDeviceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	NodeName  string `json:"nodeName"`
	Size      int64  `json:"size"`
	Type      string `json:"type"`
	RealPath  string `json:"realPath"`
	Major     uint32 `json:"major"`
	Minor     uint32 `json:"minor"`
	UUID      string `json:"uuid"`
	Available bool   `json:"available"`
}

// RawDeviceStatus defines the observed state of RawDevice
type RawDeviceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Name string `json:"name"`
}

//+genclient
//+genclient:nonNamespaced
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// RawDevice is the Schema for the rawdevices API
type RawDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RawDeviceSpec   `json:"spec,omitempty"`
	Status RawDeviceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RawDeviceList contains a list of RawDevice
type RawDeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RawDevice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RawDevice{}, &RawDeviceList{})
}
