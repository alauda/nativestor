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

import "time"

var (
	TopolvmImage          string
	CertsSecret           string
	NameSpace             string
	ClusterName           string
	CSIKubeletRootDir     string
	IsOperatorHub         bool
	EnableDiscoverDevices string
	CheckStatusInterval   time.Duration
)

const (
	// AppAttr app label
	AppAttr            = "app.kubernetes.io/name"
	TopolvmComposeAttr = "app.kubernetes.io/compose"
	TopolvmComposeNode = "node"
	// ClusterAttr cluster label
	ClusterAttr                     = "topolvm_cluster"
	NodeNameEnv                     = "NODE_NAME"
	ClusterNameEnv                  = "CLUSTER_NAME"
	PodNameSpaceEnv                 = "POD_NAMESPACE"
	PodNameEnv                      = "POD_NAME"
	LogLevelEnv                     = "TOPOLVM_LOG_LEVEL"
	UseLoopEnv                      = "USE_LOOP"
	TopolvmNodeDeploymentFmt        = "topolvm-node-%s"
	LvmdConfigMapFmt                = "lvmdconfig-%s"
	LvmdConfigMapLabelKey           = "topolvm/lvmdconfig"
	LvmdConfigMapLabelValue         = "lvmdconfig"
	LvmdConfigMapKey                = "lvmd.yaml"
	LocalDiskCMData                 = "devices"
	VgStatusConfigMapKey            = "status.json"
	LvmdAnnotationsNodeKey          = "node-name"
	LvmdSocketPath                  = "/run/topolvm/lvmd.sock"
	TopolvmDiscoverDeviceMemRequest = "50Mi"
	TopolvmDiscoverDeviceMemLimit   = "50Mi"
	TopolvmDiscoverDeviceCPURequest = "50m"
	TopolvmDiscoverDeviceCPULimit   = "50m"

	TopolvmPrepareVgMemRequest = "50Mi"
	TopolvmPrepareVgMemLimit   = "100Mi"
	TopolvmPrepareVgCPURequest = "50m"
	TopolvmPrepareVgCPULimit   = "100m"

	PrepareVgServiceAccount = "topolvm-preparevg"
	PrePareVgAppName        = "prepare-volume-group"
	PrepareVgJobFmt         = "topolvm-prepare-vg-%s"
	PrePareVgContainerName  = "preparevg"
	PrePareVgJobLogLevel    = "DEBUG"

	TopolvmNodePsp       = "topolvm-node"
	TopolvmPrepareVgPsp  = "topolvm-preparevg"
	TopolvmCSIDriverName = "topolvm.cybozu.com"

	CapacityKeyPrefix      = "capacity.topolvm.cybozu.com/"
	DiscoverDevicesAccount = "topolvm-discover"
	DiscoverContainerName  = "discover"
	UseLoop                = "true"
	LoopCreateSuccessful   = "successful"
	LoopAnnotationsKey     = "loop"
	LoopAnnotationsVal     = "true"
)
