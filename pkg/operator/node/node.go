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

package node

import (
	"context"
	"fmt"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/pkg/errors"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"strings"
)

func CheckNodeDeploymentIsExisting(clientset kubernetes.Interface, deploymentName string) (bool, error) {

	_, err := clientset.AppsV1().Deployments(cluster.NameSpace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return false, errors.Wrapf(err, "failed to detect deployment %s", deploymentName)
	} else if err == nil {
		return true, nil
	}
	return false, nil
}

func CreateReplaceDeployment(clientset kubernetes.Interface, deploymentName string, lvmdOConfigMapName string, nodeName string, ref *metav1.OwnerReference) error {

	_, err := clientset.AppsV1().Deployments(cluster.NameSpace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to detect deployment %s", deploymentName)
	} else if err == nil {
		err := k8sutil.DeleteDeployment(clientset, cluster.NameSpace, deploymentName)
		if err != nil {
			return errors.Wrapf(err, "failed to remove deployment %s ", deploymentName)
		}
	}
	return CreateNodeDeployment(clientset, deploymentName, lvmdOConfigMapName, nodeName, ref)
}

func CreateNodeDeployment(clientset kubernetes.Interface, deploymentName string, lvmdOConfigMapName string, nodeName string, ref *metav1.OwnerReference) error {

	deployment := getDeployment(deploymentName, nodeName, lvmdOConfigMapName, ref)
	if err := k8sutil.CreateDeployment(clientset, deploymentName, cluster.NameSpace, deployment); err != nil {
		return errors.Wrapf(err, "create node deployment %s failed", deploymentName)
	}
	return nil
}

func UpdateNodeDeploymentCSIKubeletRootPath(clientset kubernetes.Interface, path string) error {

	d, err := clientset.AppsV1().Deployments(cluster.NameSpace).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", cluster.AppAttr, cluster.TopolvmNodeDeploymentLabelName)})
	if err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to list topolvm node deployment")
	}

	command := []string{
		"/csi-node-driver-registrar",
		"--csi-address=/run/topolvm/csi-topolvm.sock",
		fmt.Sprintf("--kubelet-registration-path=%splugins/topolvm.cybozu.com/node/csi-topolvm.sock", getAbsoluteKubeletPath(path)),
	}

	for i := range d.Items {
		newDep := d.Items[i].DeepCopy()
		for j := range newDep.Spec.Template.Spec.Volumes {
			switch newDep.Spec.Template.Spec.Volumes[j].Name {
			case "registration-dir":
				newDep.Spec.Template.Spec.Volumes[j].VolumeSource.HostPath.Path = fmt.Sprintf("%splugins_registry/", getAbsoluteKubeletPath(path))
			case "node-plugin-dir":
				newDep.Spec.Template.Spec.Volumes[j].VolumeSource.HostPath.Path = fmt.Sprintf("%splugins/topolvm.cybozu.com/node", getAbsoluteKubeletPath(path))
			case "csi-plugin-dir":
				newDep.Spec.Template.Spec.Volumes[j].VolumeSource.HostPath.Path = fmt.Sprintf("%splugins/kubernetes.io/csi", getAbsoluteKubeletPath(path))
			case "pod-volumes-dir":
				newDep.Spec.Template.Spec.Volumes[j].VolumeSource.HostPath.Path = fmt.Sprintf("%spods/", getAbsoluteKubeletPath(path))
			}
		}

		for j := range newDep.Spec.Template.Spec.Containers {
			if newDep.Spec.Template.Spec.Containers[j].Name == cluster.CsiRegistrarContainerName {
				newDep.Spec.Template.Spec.Containers[j].Command = command
			}
		}

		patchChanged := false
		patchResult, err := patch.DefaultPatchMaker.Calculate(&d.Items[i], newDep)
		if err != nil {
			patchChanged = true
		} else if !patchResult.IsEmpty() {
			patchChanged = true
		}

		if !patchChanged {
			continue
		}

		if _, err := clientset.AppsV1().Deployments(cluster.NameSpace).Update(context.TODO(), newDep, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update deployment %q. %v", newDep.Name, err)
		}

	}
	return nil
}

func getDeployment(appName string, nodeName string, congfigmap string, ref *metav1.OwnerReference) *v1.Deployment {

	replicas := int32(1)
	hostPathDirectory := corev1.HostPathDirectory
	hostPathDirectoryOrCreateType := corev1.HostPathDirectoryOrCreate
	storageMedium := corev1.StorageMediumMemory

	volumes := []corev1.Volume{
		{Name: "registration-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: fmt.Sprintf("%splugins_registry/", getAbsoluteKubeletPath(cluster.CSIKubeletRootDir)), Type: &hostPathDirectory}}},
		{Name: "node-plugin-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: fmt.Sprintf("%splugins/topolvm.cybozu.com/node", getAbsoluteKubeletPath(cluster.CSIKubeletRootDir)), Type: &hostPathDirectoryOrCreateType}}},
		{Name: "csi-plugin-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: fmt.Sprintf("%splugins/kubernetes.io/csi", getAbsoluteKubeletPath(cluster.CSIKubeletRootDir)), Type: &hostPathDirectoryOrCreateType}}},
		{Name: "pod-volumes-dir", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: fmt.Sprintf("%spods/", getAbsoluteKubeletPath(cluster.CSIKubeletRootDir)), Type: &hostPathDirectoryOrCreateType}}},
		{Name: "lvmd-config-dir", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: congfigmap}}}},
		{Name: "lvmd-socket-dir", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: storageMedium}}},
	}

	containers := []corev1.Container{*getLvmdContainer(), *getNodeContainer(), *getCsiRegistrarContainer(), *getLivenessProbeContainer()}

	nodeDeployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            appName,
			Namespace:       cluster.NameSpace,
			OwnerReferences: []metav1.OwnerReference{*ref},
			Labels:          map[string]string{cluster.AppAttr: cluster.TopolvmNodeDeploymentLabelName},
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					cluster.AppAttr: appName,
				},
			},
			Strategy: v1.DeploymentStrategy{Type: v1.RecreateDeploymentStrategyType},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: appName,
					Labels: map[string]string{
						cluster.AppAttr:            appName,
						cluster.TopolvmComposeAttr: cluster.TopolvmComposeNode,
					},
				},
				Spec: corev1.PodSpec{
					Containers:         containers,
					ServiceAccountName: cluster.NodeServiceAccount,
					Volumes:            volumes,
					HostPID:            true,
					NodeSelector:       map[string]string{corev1.LabelHostname: nodeName},
					Tolerations:        []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
				},
			},
		},
	}
	return nodeDeployment

}

func getLvmdContainer() *corev1.Container {

	command := []string{
		"/lvmd",
		"--config=/etc/topolvm/lvmd.yaml",
		"--container=true",
	}

	resourceRequirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmNodeCPULimit),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmNodeMemLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmNodeCPURequest),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmNodeMemRequest),
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "lvmd-socket-dir", MountPath: "/run/topolvm"},
		{Name: "lvmd-config-dir", MountPath: "/etc/topolvm"}}

	lvmd := &corev1.Container{
		Name:            cluster.LvmdContainerName,
		Image:           cluster.TopolvmImage,
		SecurityContext: getPrivilegeSecurityContext(),
		Command:         command,
		Resources:       resourceRequirements,
		VolumeMounts:    volumeMounts,
	}
	return lvmd
}

func getNodeContainer() *corev1.Container {

	command := []string{
		"/topolvm-node",
		"--lvmd-socket=/run/lvmd/lvmd.sock",
	}

	requirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmNodeCPULimit),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmNodeMemLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cluster.TopolvmNodeCPURequest),
			corev1.ResourceMemory: resource.MustParse(cluster.TopolvmNodeMemRequest),
		},
	}

	mountPropagationMode := corev1.MountPropagationBidirectional

	volumeMounts := []corev1.VolumeMount{
		{Name: "node-plugin-dir", MountPath: "/run/topolvm"},
		{Name: "lvmd-socket-dir", MountPath: "/run/lvmd"},
		{Name: "pod-volumes-dir", MountPath: "/var/lib/kubelet/pods", MountPropagation: &mountPropagationMode},
		{Name: "csi-plugin-dir", MountPath: "/var/lib/kubelet/plugins/kubernetes.io/csi", MountPropagation: &mountPropagationMode},
	}

	env := []corev1.EnvVar{
		{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
	}

	node := &corev1.Container{
		Name:            cluster.NodeContainerName,
		Image:           cluster.TopolvmImage,
		Command:         command,
		SecurityContext: getPrivilegeSecurityContext(),
		Ports:           []corev1.ContainerPort{{Name: cluster.TopolvmNodeContainerHealthzName, ContainerPort: 9808, Protocol: corev1.ProtocolTCP}},
		LivenessProbe: &corev1.Probe{Handler: corev1.Handler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromString(cluster.TopolvmNodeContainerHealthzName)}},
			FailureThreshold: 3, InitialDelaySeconds: 10, TimeoutSeconds: 3, PeriodSeconds: 60},
		Resources:    requirements,
		Env:          env,
		VolumeMounts: volumeMounts,
	}
	return node
}

func getCsiRegistrarContainer() *corev1.Container {

	command := []string{
		"/csi-node-driver-registrar",
		"--csi-address=/run/topolvm/csi-topolvm.sock",
		fmt.Sprintf("--kubelet-registration-path=%splugins/topolvm.cybozu.com/node/csi-topolvm.sock", getAbsoluteKubeletPath(cluster.CSIKubeletRootDir)),
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "node-plugin-dir", MountPath: "/run/topolvm"},
		{Name: "registration-dir", MountPath: "/registration"},
	}

	preStopCmd := []string{
		"/bin/sh",
		"-c",
		"rm -rf /registration/topolvm.cybozu.com /registration/topolvm.cybozu.com-reg.sock",
	}

	csi := &corev1.Container{
		Name:         cluster.CsiRegistrarContainerName,
		Image:        cluster.TopolvmImage,
		Command:      command,
		Lifecycle:    &corev1.Lifecycle{PreStop: &corev1.Handler{Exec: &corev1.ExecAction{Command: preStopCmd}}},
		VolumeMounts: volumeMounts,
	}
	return csi
}

func getLivenessProbeContainer() *corev1.Container {

	command := []string{
		"/livenessprobe",
		"--csi-address=/run/topolvm/csi-topolvm.sock",
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: "node-plugin-dir", MountPath: "/run/topolvm"},
	}

	liveness := &corev1.Container{
		Name:         cluster.TopolvmCsiLivenessProbeContainerName,
		Image:        cluster.TopolvmImage,
		Command:      command,
		VolumeMounts: volumeMounts,
	}
	return liveness
}

func getPrivilegeSecurityContext() *corev1.SecurityContext {
	privilege := true
	runUser := int64(0)
	return &corev1.SecurityContext{Privileged: &privilege, RunAsUser: &runUser}
}

func getAbsoluteKubeletPath(name string) string {
	if strings.HasSuffix(name, "/") {
		return name
	} else {
		return name + "/"
	}
}
