/*
Copyright 2021 The Topolvm-Operator Authors. All rights reserved.

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

package topolvm

type LmvdConf struct {
	SocketName    string        `yaml:"socket-name"`
	DeviceClasses []DeviceClass `yaml:"device-classes"`
}

type DeviceClass struct {
	Name        string `yaml:"name"`
	VolumeGroup string `yaml:"volume-group"`
	SpareGb     uint64 `yaml:"spare-gb,omitempty"`
	Default     bool   `yaml:"default"`
	Stripe      uint   `yaml:"stripe,omitempty"`
	StripeSize  string `yaml:"stripe-size,omitempty"`
}

type Metrics struct {
	Cluster       string
	ClusterStatus uint8
	NodeStatus    []NodeStatusMetrics
}

type NodeStatusMetrics struct {
	Node   string
	Status uint8
}

type OperatorConfig struct {
	Parameters map[string]string
	Image      string
}
