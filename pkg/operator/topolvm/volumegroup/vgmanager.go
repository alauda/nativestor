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
	"github.com/alauda/topolvm-operator/pkg/cluster/topolvm"

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

const (
	indexJobMajorNumber = "1"
	// from k8s v1.21 job spec can have a field name 'completionMode' which is
	// in alpha and in v1.22 it's in beta
	indexJobMinorNumber = "22"
)

func makeJob(clientset kubernetes.Interface, nodeName string, image string, reference *metav1.OwnerReference) (*batch.Job, error) {

	podSpec, err := provisionPodTemplateSpec(nodeName, image, v1.RestartPolicyNever)
	if err != nil {
		return nil, err
	}

	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			// job validation introduced in v1.22 uses DNS regex which accepts only
			// domain name but not subdomains, so truncating the name to always pass hash
			Name:      truncateNodeNameForIndexJob(topolvm.PrepareVgJobFmt, nodeName),
			Namespace: topolvm.NameSpace,
			Labels: map[string]string{
				topolvm.AppAttr:     topolvm.PrePareVgAppName,
				topolvm.ClusterAttr: topolvm.NameSpace,
			},
			OwnerReferences: []metav1.OwnerReference{*reference},
		},
	}

	jobSpec := batch.JobSpec{
		Template: *podSpec,
	}

	if version.Major >= indexJobMajorNumber && version.Minor >= indexJobMinorNumber {
		// workaround for https://github.com/kubernetes/kubernetes/pull/105676
		// can be removed after merge of above PR, in v1.21 feature gate has to
		// be enabled so keeping minor number as 22 see below for more info
		// https://kubernetes.io/blog/2021/04/19/introducing-indexed-jobs/
		indexed := batch.CompletionMode(batch.IndexedCompletion)
		completions := int32(1)
		parallelism := int32(1)
		jobSpec.CompletionMode = &indexed
		jobSpec.Completions = &completions
		jobSpec.Parallelism = &parallelism
	}
	job.Spec = jobSpec
	return job, nil
}

func truncateNodeNameForIndexJob(format, nodeName string) string {
	hashed := k8sutil.Hash(nodeName)
	logger.Infof("nodeName %s will be %s", nodeName, hashed)
	return fmt.Sprintf(format, hashed)
}

func MakeAndRunJob(clientset kubernetes.Interface, nodeName string, image string, reference *metav1.OwnerReference) error {
	// update the orchestration status of this node to the starting state

	logger.Debugf("start make prepare vg job")
	job, err := makeJob(clientset, nodeName, image, reference)
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
			v1.ResourceCPU:    resource.MustParse(topolvm.TopolvmPrepareVgCPULimit),
			v1.ResourceMemory: resource.MustParse(topolvm.TopolvmPrepareVgMemLimit),
		},
		Requests: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(topolvm.TopolvmPrepareVgCPURequest),
			v1.ResourceMemory: resource.MustParse(topolvm.TopolvmPrepareVgMemRequest),
		},
	}

	podSpec := v1.PodSpec{
		ServiceAccountName: topolvm.PrepareVgServiceAccount,
		Containers: []v1.Container{
			{
				Name:      topolvm.PrePareVgContainerName,
				Image:     image,
				Command:   command,
				Resources: resourceRequirements,
				SecurityContext: &v1.SecurityContext{
					Privileged: &privileged,
					RunAsUser:  &runAsUser,
				},
				VolumeMounts: volumeMount,
				Env: []v1.EnvVar{
					{Name: topolvm.NodeNameEnv, Value: nodeName},
					{Name: topolvm.PodNameSpaceEnv, Value: topolvm.NameSpace},
					{Name: topolvm.LogLevelEnv, Value: topolvm.PrePareVgJobLogLevel},
					{Name: topolvm.ClusterNameEnv, Value: topolvm.ClusterName},
				},
			},
		},
		Tolerations:   []v1.Toleration{{Operator: v1.TolerationOpExists}},
		NodeSelector:  map[string]string{v1.LabelHostname: nodeName},
		RestartPolicy: restart,
		Volumes:       volumes,
		HostIPC:       true,
		HostPID:       true,
	}

	podMeta := metav1.ObjectMeta{
		Name: topolvm.PrePareVgAppName,
		Labels: map[string]string{
			topolvm.AppAttr:     topolvm.PrePareVgAppName,
			topolvm.ClusterAttr: topolvm.NameSpace,
		},
		Annotations: map[string]string{},
	}
	return &v1.PodTemplateSpec{
		ObjectMeta: podMeta,
		Spec:       podSpec,
	}, nil

}
