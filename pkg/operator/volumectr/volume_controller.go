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

package volumectr

import (
	"context"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/coreos/pkg/capnslog"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

var (
	logger = capnslog.NewPackageLogger("topolvm/operator", "controller-deployment")
)

func CreateReplaceTopolvmControllerDeployment(clientset kubernetes.Interface, ref *metav1.OwnerReference) error {

	deploymentName := cluster.TopolvmControllerDeploymentName
	_, err := clientset.AppsV1().Deployments(cluster.NameSpace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		logger.Errorf("failed to detect deployment:%s. err:%v", deploymentName, err)
		return err
	} else if err == nil {

		err := k8sutil.DeleteDeployment(clientset, cluster.NameSpace, deploymentName)
		if err != nil {
			logger.Errorf("failed to remove deployment:%s. err:%v", deploymentName, err)
			return err
		}
	}
	return CreateControllerDeployment(clientset, ref)
}

func CreateControllerDeployment(clientset kubernetes.Interface, ref *metav1.OwnerReference) error {

	deployment := getDeployment(ref)

	if err := k8sutil.CreateDeployment(clientset, cluster.TopolvmControllerDeploymentName, cluster.NameSpace, deployment); err != nil {
		logger.Errorf("create node deployment %s failed err %s", cluster.TopolvmControllerDeploymentName, err)
		return err
	}
	return nil
}

func getDeployment(ref *metav1.OwnerReference) *v1.Deployment {

	replicas := int32(2)

	volumes := []corev1.Volume{
		{Name: "socket-dir", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}

	containers := []corev1.Container{*getContorllerContainer(), *getCsiProvisionerContainer(), *getCsiAttacherContainer(), *getCsiResizerContainer(), *getCsiSnapShotterContainer(), *getLivenessProbeContainer()}

	controllerDeployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            cluster.TopolvmControllerDeploymentName,
			Namespace:       cluster.NameSpace,
			OwnerReferences: []metav1.OwnerReference{*ref},
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					cluster.AppAttr: cluster.TopolvmControllerDeploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cluster.TopolvmControllerDeploymentName,
					Namespace: cluster.NameSpace,
					Labels: map[string]string{
						cluster.AppAttr: cluster.TopolvmControllerDeploymentName,
					},
				},
				Spec: corev1.PodSpec{
					Containers:         containers,
					ServiceAccountName: cluster.ContollerServiceAccount,
					Volumes:            volumes,
					Tolerations:        []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
				},
			},
		},
	}
	return controllerDeployment

}

func getContorllerContainer() *corev1.Container {

	command := []string{
		"/topolvm-controller",
		"--cert-dir=/certs",
	}

	resourceRequirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmControllerCPULimit),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmControllerMemLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmControllerCPURequest),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmControllerMemRequest),
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "socket-dir", MountPath: "/run/topolvm"},
	}

	controller := &corev1.Container{
		Name:    cluster.TopolvmControllerContainerName,
		Image:   cluster.TopolvmImage,
		Command: command,
		Ports:   []corev1.ContainerPort{{Name: cluster.TopolvmControllerContainerHealthzName, ContainerPort: cluster.TopolvmControllerContainerLivenessPort, Protocol: corev1.ProtocolTCP}},
		LivenessProbe: &corev1.Probe{Handler: corev1.Handler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromString(cluster.TopolvmControllerContainerHealthzName)}},
			FailureThreshold: 3, InitialDelaySeconds: 10, TimeoutSeconds: 3, PeriodSeconds: 60},
		ReadinessProbe: &corev1.Probe{Handler: corev1.Handler{HTTPGet: &corev1.HTTPGetAction{Path: "/metrics", Port: intstr.IntOrString{IntVal: cluster.TopolvmControllerContainerReadinessPort}, Scheme: corev1.URISchemeHTTP}}},
		Resources:      resourceRequirements,
		VolumeMounts:   volumeMounts,
	}
	return controller
}

func getCsiResizerContainer() *corev1.Container {

	command := []string{
		"/csi-resizer",
		"--csi-address=/run/topolvm/csi-topolvm.sock",
		"--leader-election",
		"--leader-election-namespace=" + cluster.NameSpace,
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "socket-dir", MountPath: "/run/topolvm"},
	}

	csiResizer := &corev1.Container{
		Name:         cluster.TopolvmCsiResizerContainerName,
		Image:        cluster.TopolvmImage,
		Command:      command,
		VolumeMounts: volumeMounts,
	}
	return csiResizer
}

func getCsiAttacherContainer() *corev1.Container {

	command := []string{
		"/csi-attacher",
		"--csi-address=/run/topolvm/csi-topolvm.sock",
		"--leader-election",
		"--leader-election-namespace=" + cluster.NameSpace,
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "socket-dir", MountPath: "/run/topolvm"},
	}

	csiAttacher := &corev1.Container{
		Name:         cluster.TopolvmCsiAttacherContainerName,
		Image:        cluster.TopolvmImage,
		Command:      command,
		VolumeMounts: volumeMounts,
	}
	return csiAttacher
}

func getCsiProvisionerContainer() *corev1.Container {

	command := []string{"/csi-provisioner",
		"--csi-address=/run/topolvm/csi-topolvm.sock",
		"--enable-capacity",
		"--capacity-ownerref-level=2",
		"--capacity-poll-interval=30s",
		"--feature-gates=Topology=true",
		"--leader-election",
		"--leader-election-namespace=" + cluster.NameSpace,
	}

	resourceRequirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmControllerCsiProvisionCPULimit),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmControllerCsiProvisionMemLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmControllerCsiProvisionCPURequest),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmControllerCsiProvisionMemRequest),
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "socket-dir", MountPath: "/run/topolvm"},
	}

	env := []corev1.EnvVar{
		{Name: cluster.PodNameSpaceEnv, ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
		{Name: cluster.PodNameEnv, ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
	}

	csiProvisoiner := &corev1.Container{
		Name:         cluster.TopolvmCsiProvisionerContainerName,
		Image:        cluster.TopolvmImage,
		Command:      command,
		Resources:    resourceRequirements,
		VolumeMounts: volumeMounts,
		Env:          env,
	}
	return csiProvisoiner
}

func getCsiSnapShotterContainer() *corev1.Container {

	command := []string{"/csi-snapshotter",
		"--csi-address=/run/topolvm/csi-topolvm.sock",
		"--leader-election",
		"--leader-election-namespace=" + cluster.NameSpace,
	}

	resourceRequirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmControllerCsiSnapShotterCPULimit),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmControllerCsiSnapShotterMemLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmControllerCsiSnapShotterCPURequest),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmControllerCsiSnapShotterMemRequest),
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "socket-dir", MountPath: "/run/topolvm"},
	}

	csiSnapShotter := &corev1.Container{
		Name:         cluster.TopolvmCsiSnapShotterContainerName,
		Image:        cluster.TopolvmImage,
		Command:      command,
		Resources:    resourceRequirements,
		VolumeMounts: volumeMounts,
	}
	return csiSnapShotter
}

func getLivenessProbeContainer() *corev1.Container {

	command := []string{
		"/livenessprobe",
		"--csi-address=/run/topolvm/csi-topolvm.sock",
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "socket-dir", MountPath: "/run/topolvm"},
	}

	livenessProbe := &corev1.Container{
		Name:         cluster.TopolvmCsiLivenessProbeContainerName,
		Image:        cluster.TopolvmImage,
		Command:      command,
		VolumeMounts: volumeMounts,
	}
	return livenessProbe
}
