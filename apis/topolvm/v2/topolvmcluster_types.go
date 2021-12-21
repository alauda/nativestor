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

package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TopolvmClusterSpec defines the desired state of TopolvmCluster
type TopolvmClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	TopolvmVersion string `json:"topolvmVersion"`
	//+optional
	CertsSecret string `json:"certsSecret"`
	Storage     `json:"storage"`
}

type Storage struct {
	DeviceClasses   []NodeDevices `json:"deviceClasses,omitempty"`
	UseAllNodes     bool          `json:"useAllNodes"`
	UseAllDevices   bool          `json:"useAllDevices"`
	Devices         []Disk        `json:"devices,omitempty"`
	VolumeGroupName string        `json:"volumeGroupName,omitempty"`
	ClassName       string        `json:"className,omitempty"`
	UseLoop         bool          `json:"useLoop"`
}

type NodeDevices struct {
	NodeName      string        `json:"nodeName"`
	DeviceClasses []DeviceClass `json:"classes"`
}

type DeviceClass struct {
	ClassName  string `json:"className" yaml:"name"`
	VgName     string `json:"volumeGroup" yaml:"volume-group"`
	Device     []Disk `json:"devices" yaml:"devices,omitempty"`
	Default    bool   `json:"default,omitempty" yaml:"default,omitempty"`
	SpareGb    uint64 `json:"spareGb,omitempty" yaml:"spare-gb,omitempty"`
	Stripe     uint   `json:"stripe,omitempty" yaml:"stripe,omitempty"`
	StripeSize string `json:"stripeSize,omitempty" yaml:"stripe-size,omitempty"`
}

type Disk struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Auto bool   `json:"auto,omitempty"`
	Path string `json:"path,omitempty"`
	Size uint64 `json:"size,omitempty"`
}

// TopolvmClusterStatus defines the observed state of TopolvmCluster
type TopolvmClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase             ConditionType      `json:"phase"`
	NodeStorageStatus []NodeStorageState `json:"nodeStorageState"`
}

type ConditionType string

const (
	ConditionReady   ConditionType = "Ready"
	ConditionFailure ConditionType = "Failure"
	ConditionUnknown ConditionType = "Unknown"
	ConditionPending ConditionType = "Pending"
)

type NodeStorageState struct {
	Node  string        `json:"node"`
	Phase ConditionType `json:"phase"`
	//+optional
	FailClasses []ClassState `json:"failClasses"`
	//+optional
	SuccessClasses []ClassState `json:"successClasses"`
	//+optional
	Loops []LoopState `json:"loops"`
}

type LoopState struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	DeviceName string `json:"deviceName"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

type ClassStateType string

const (
	ClassReady   ClassStateType = "Ready"
	ClassUnReady ClassStateType = "UnReady"
)

type ClassState struct {
	Name         string         `json:"className,omitempty"`
	VgName       string         `json:"vgName,omitempty"`
	State        ClassStateType `json:"state,omitempty"`
	Message      string         `json:"message,omitempty"`
	DeviceStates []DeviceState  `json:"deviceStates,omitempty"`
}

type DeviceStateType string

const (
	DeviceOnline  DeviceStateType = "Online"
	DeviceOffline DeviceStateType = "Offline"
)

type DeviceState struct {
	Name    string          `json:"name,omitempty"`
	State   DeviceStateType `json:"state,omitempty"`
	Message string          `json:"message,omitempty"`
}

//+genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TopolvmCluster is the Schema for the topolvmclusters API
type TopolvmCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TopolvmClusterSpec   `json:"spec,omitempty"`
	Status TopolvmClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TopolvmClusterList contains a list of TopolvmCluster
type TopolvmClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TopolvmCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TopolvmCluster{}, &TopolvmClusterList{})
}
