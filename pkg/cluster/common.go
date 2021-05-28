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

package cluster

import "time"

var (
	TopolvmImage  string
	NameSpace     string
	ClusterName   string
	IsOperatorHub string
)

const (
	// AppAttr app label
	AppAttr            = "app.kubernetes.io/name"
	TopolvmComposeAttr = "app.kubernetes.io/compose"
	TopolvmComposeNode = "node"

	// ClusterAttr cluster label
	ClusterAttr = "topolvm_cluster"

	NodeNameEnv                     = "NODE_NAME"
	ClusterNameEnv                  = "CLUSTER_NAME"
	PodNameSpaceEnv                 = "POD_NAMESPACE"
	IsOperatorHubEnv                = "IS_OPERATOR_HUB"
	PodNameEnv                      = "POD_NAME"
	LogLevelEnv                     = "TOPOLVM_LOG_LEVEL"
	UseLoopEnv                      = "USE_LOOP"
	CheckStatusIntervalEnv          = "CHECK_STATUS_INTERVAL"
	DefaultCheckStatusInterval      = 5 * time.Second
	TopolvmNodeDeploymentNamePrefix = "topolvm-node-"
	NodeServiceAccount              = "topolvm-node"
	TopolvmNodeDeploymentLabelName  = "topolvm-node"

	LvmdConfigMapNamePrefix = "lvmdconfig"
	LvmdConfigMapFmt        = "lvmdconfig-%s"
	LvmdConfigMapLabelKey   = "topolvm/lvmdconfig"
	LvmdConfigMapLabelValue = "lvmdconfig"
	LvmdConfigMapKey        = "lvmd.yaml"
	LocalDiskCMData         = "devices"
	VgStatusConfigMapKey    = "status.json"
	LvmdAnnotationsNodeKey  = "node-name"
	LvmdSocketPath          = "/run/topolvm/lvmd.sock"

	ContollerServiceAccount = "topolvm-controller"
	LvmdContainerName       = "lvmd"
	NodeContainerName       = "topolvm-node"

	CsiRegistrarContainerName = "csi-registrar"

	TopolvmControllerDeploymentName      = "topolvm-controller"
	TopolvmControllerContainerName       = "topolvm-controller"
	TopolvmControllerSecrectsName        = "topolvm-mutatingwebhook"
	TopolvmCsiResizerContainerName       = "csi-resizer"
	TopolvmCsiAttacherContainerName      = "csi-attacher"
	TopolvmCsiProvisionerContainerName   = "csi-provisioner"
	TopolvmCsiLivenessProbeContainerName = "liveness-probe"

	TopolvmControllerContainerHealthzName   = "healthz"
	TopolvmNodeContainerHealthzName         = "healthz"
	TopolvmControllerContainerLivenessPort  = int32(9808)
	TopolvmControllerContainerReadinessPort = int32(8080)

	TopolvmNodeMemRequest = "250Mi"
	TopolvmNodeMemLimit   = "250Mi"
	TopolvmNodeCPURequest = "250m"
	TopolvmNodeCPULimit   = "250m"

	TopolvmControllerMemRequest = "250Mi"
	TopolvmControllerMemLimit   = "250Mi"
	TopolvmControllerCPURequest = "250m"
	TopolvmControllerCPULimit   = "250m"

	TopolvmControllerCsiProvisionMemRequest = "250Mi"
	TopolvmControllerCsiProvisionMemLimit   = "250Mi"
	TopolvmControllerCsiProvisionCPURequest = "250m"
	TopolvmControllerCsiProvisionCPULimit   = "250m"

	TopolvmDiscoverDeviceMemRequest = "50Mi"
	TopolvmDiscoverDeviceMemLimit   = "50Mi"
	TopolvmDiscoverDeviceCPURequest = "50m"
	TopolvmDiscoverDeviceCPULimit   = "50m"

	TopolvmPrepareVgMemRequest = "50Mi"
	TopolvmPrepareVgMemLimit   = "100Mi"
	TopolvmPrepareVgCPURequest = "50m"
	TopolvmPrepareVgCPULimit   = "100m"

	PrepareVgServiceAccount = "topolvm-preparevg"
	PrePareVgAppName        = "prepareVolumeGroup"
	PrepareVgJobFmt         = "topolvm-prepare-vg-%s"
	PrePareVgContainerName  = "preparevg"
	PrePareVgJobLogLevel    = "DEBUG"

	TopolvmNodePsp      = "topolvm-node"
	TopolvmPrepareVgPsp = "topolvm-preparevg"

	TopolvmCSIDriverName = "topolvm.cybozu.com"
	IsOperator           = "1"
	CapacityKeyPrefix    = "capacity.topolvm.cybozu.com/"
	NodeAttr             = "topolvm/node"

	DiscoverDevicesAccount = "topolvm-discover"
	DiscoverAppName        = "topolvm-discover"
	DiscoverContainerName  = "discover"
	UseLoop                = "1"
	LoopCreateSuccessful   = "successful"
)
