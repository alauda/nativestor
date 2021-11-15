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

package lvmd

import (
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	rookutils "github.com/rook/rook/pkg/operator/k8sutil"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	logger = capnslog.NewPackageLogger("topolvm/operator", "lvmd")
)

func MakeLvmdDaemonSet(clientSet kubernetes.Interface, lvmdConfigMapName string) error {

	daemon := getDaemonSet(cluster.LvmdAppName, lvmdConfigMapName)

	operatorPod, err := k8sutil.GetRunningPod(clientSet)
	if err != nil {
		logger.Errorf("failed to get operator pod. %+v", err)
	} else {
		rookutils.SetOwnerRefsWithoutBlockOwner(&daemon.ObjectMeta, operatorPod.OwnerReferences)
	}
	if err := k8sutil.CreateDaemonSet(cluster.LvmdAppName, cluster.NameSpace, clientSet, daemon); err != nil {
		return errors.Wrapf(err, "create daemonSet  %s failed", cluster.LvmdAppName)
	}
	return nil
}

func getDaemonSet(appName string, lvmdConfigMapName string) *v1.DaemonSet {

	privileged := true
	runAsUser := int64(0)
	command := []string{
		"/lvmd",
		"--config=/etc/topolvm/lvmd.yaml",
		"--container=true",
	}

	resourceRequirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmLvmdCPULimit),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmLvmdMemLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmLvmdCPURequest),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmLvmdMemRequest),
		},
	}

	storageMedium := corev1.StorageMediumMemory
	volumes := []corev1.Volume{
		{Name: "lvmd-config-dir", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: lvmdConfigMapName}}}},
		{Name: "lvmd-socket-dir", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: storageMedium}}},
	}
	volumeMounts := []corev1.VolumeMount{
		{Name: "lvmd-socket-dir", MountPath: "/run/topolvm"},
		{Name: "lvmd-config-dir", MountPath: "/etc/topolvm"},
	}

	env := []corev1.EnvVar{
		k8sutil.NamespaceEnvVar(),
		k8sutil.NodeEnvVar(),
		k8sutil.NameEnvVar(),
	}

	daemonSet := &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: cluster.NameSpace,
		},
		Spec: v1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					cluster.AppAttr: appName,
				},
			},
			UpdateStrategy: v1.DaemonSetUpdateStrategy{
				Type: v1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: appName,
					Labels: map[string]string{
						cluster.AppAttr: appName,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: cluster.NodeServiceAccount,
					Containers: []corev1.Container{
						{
							Name:      cluster.LvmdContainerName,
							Image:     cluster.TopolvmImage,
							Command:   command,
							Resources: resourceRequirements,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
								RunAsUser:  &runAsUser,
							},
							VolumeMounts: volumeMounts,
							Env:          env,
						},
					},
					Volumes:     volumes,
					HostPID:     true,
					Tolerations: []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
				},
			},
		},
	}
	return daemonSet
}
