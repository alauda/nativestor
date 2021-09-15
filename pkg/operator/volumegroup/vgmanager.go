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

package volumegroup

import (
	"fmt"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/coreos/pkg/capnslog"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	logger = capnslog.NewPackageLogger("topolvm/operator", "volume-group")
)

func makeJob(nodeName string, image string, reference *metav1.OwnerReference) (*batch.Job, error) {

	podSpec, err := provisionPodTemplateSpec(nodeName, image, v1.RestartPolicyNever)
	if err != nil {
		return nil, err
	}

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sutil.TruncateNodeName(cluster.PrepareVgJobFmt, nodeName),
			Namespace: cluster.NameSpace,
			Labels: map[string]string{
				cluster.AppAttr:     cluster.PrePareVgAppName,
				cluster.ClusterAttr: cluster.NameSpace,
			},
			OwnerReferences: []metav1.OwnerReference{*reference},
		},
		Spec: batch.JobSpec{
			Template: *podSpec,
		},
	}
	return job, nil

}

func MakeAndRunJob(clientset kubernetes.Interface, nodeName string, image string, reference *metav1.OwnerReference) error {
	// update the orchestration status of this node to the starting state

	logger.Debugf("start make prepare vg job")
	job, err := makeJob(nodeName, image, reference)
	if err != nil {
		logger.Errorf("make job for node:%s failed", nodeName)
		return err
	}

	if err := runJob(clientset, job, nodeName); err != nil {
		logger.Errorf("run job for node:%s failed", nodeName)
		return fmt.Errorf("run job for node:%s failed", nodeName)
	}
	return nil
}

func runJob(clientset kubernetes.Interface, job *batch.Job, nodeName string) error {
	if err := k8sutil.RunReplaceableJob(clientset, job, false); err != nil {
		logger.Errorf("run job failed for node:%s err:%v", nodeName, err)
		return err
	}
	return nil
}

func provisionPodTemplateSpec(nodeName string, image string, restart v1.RestartPolicy) (*v1.PodTemplateSpec, error) {

	var volumes []v1.Volume
	devVolume := v1.Volume{Name: "devices", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/dev"}}}
	volumes = append(volumes, devVolume)
	udevVolume := v1.Volume{Name: "udev", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/run/udev"}}}
	volumes = append(volumes, udevVolume)

	volumeMount := []v1.VolumeMount{
		{Name: "devices", MountPath: "/dev"},
		{Name: "udev", MountPath: "/run/udev"},
	}

	command := []string{"/topolvm", "prepareVg"}

	privileged := true
	runAsUser := int64(0)

	resourceRequirements := v1.ResourceRequirements{
		Limits: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(cluster.TopolvmPrepareVgCPULimit),
			v1.ResourceMemory: resource.MustParse(cluster.TopolvmPrepareVgMemLimit),
		},
		Requests: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(cluster.TopolvmPrepareVgCPURequest),
			v1.ResourceMemory: resource.MustParse(cluster.TopolvmPrepareVgMemRequest),
		},
	}

	podSpec := v1.PodSpec{
		ServiceAccountName: cluster.PrepareVgServiceAccount,
		Containers: []v1.Container{
			{
				Name:      cluster.PrePareVgContainerName,
				Image:     image,
				Command:   command,
				Resources: resourceRequirements,
				SecurityContext: &v1.SecurityContext{
					Privileged: &privileged,
					RunAsUser:  &runAsUser,
				},
				VolumeMounts: volumeMount,
				Env: []v1.EnvVar{
					{Name: cluster.NodeNameEnv, Value: nodeName},
					{Name: cluster.PodNameSpaceEnv, Value: cluster.NameSpace},
					{Name: cluster.LogLevelEnv, Value: cluster.PrePareVgJobLogLevel},
					{Name: cluster.ClusterNameEnv, Value: cluster.ClusterName},
				},
			},
		},
		Tolerations:   []v1.Toleration{{Operator: v1.TolerationOpExists}},
		NodeSelector:  map[string]string{v1.LabelHostname: nodeName},
		RestartPolicy: restart,
		Volumes:       volumes,
		NodeName:      nodeName,
		HostIPC:       true,
		HostPID:       true,
	}

	podMeta := metav1.ObjectMeta{
		Name: cluster.PrePareVgAppName,
		Labels: map[string]string{
			cluster.AppAttr:     cluster.PrePareVgAppName,
			cluster.ClusterAttr: cluster.NameSpace,
		},
		Annotations: map[string]string{},
	}
	return &v1.PodTemplateSpec{
		ObjectMeta: podMeta,
		Spec:       podSpec,
	}, nil

}
