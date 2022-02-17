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

package discover

import (
	"context"
	"github.com/alauda/nativestor/pkg/cluster"
	"github.com/alauda/nativestor/pkg/cluster/topolvm"
	"github.com/alauda/nativestor/pkg/operator"
	controllerutil "github.com/alauda/nativestor/pkg/operator/controller"
	"github.com/alauda/nativestor/pkg/operator/csi"
	"github.com/alauda/nativestor/pkg/operator/k8sutil"
	_ "github.com/coreos/pkg/capnslog"
	"github.com/pkg/errors"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName                = "discover-device"
	discoverDeviceTolerationsEnv  = "DISCOVER_DEVICE_TOLERATIONS"
	discoverDeviceNodeAffinityEnv = "DISCOVER_DEVICE_NODE_AFFINITY"
	discoverDeviceResource        = "DISCOVER_DEVICE_RESOURCE"
)

func Add(mgr manager.Manager, context *cluster.Context, opManagerContext context.Context, opConfig operator.OperatorConfig) error {
	return add(mgr, newReconciler(mgr, context, opManagerContext, opConfig))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, context *cluster.Context, opManagerContext context.Context, opConfig operator.OperatorConfig) reconcile.Reconciler {

	c := &DiscoverDeviceController{
		client:           mgr.GetClient(),
		context:          context,
		opConfig:         opConfig,
		opManagerContext: opManagerContext,
	}
	err := c.startDiscoverDeviceDaemonset()
	if err != nil {
		logger.Errorf("start discover device daemonset failed %v", err)
	}
	return c
}

type DiscoverDeviceController struct {
	client           client.Client
	context          *cluster.Context
	opManagerContext context.Context
	opConfig         operator.OperatorConfig
}

func (r *DiscoverDeviceController) Reconcile(context context.Context, request reconcile.Request) (reconcile.Result, error) {
	reconcileResponse, err := r.reconcile(request)
	if err != nil {
		logger.Errorf("failed to reconcile %v", err)
	}
	return reconcileResponse, err
}
func (r *DiscoverDeviceController) reconcile(request reconcile.Request) (reconcile.Result, error) {

	err := r.startDiscoverDeviceDaemonset()
	if err != nil {
		return controllerutil.ImmediateRetryResult, errors.Wrap(err, "failed configure discover device daemonset")
	}
	return reconcile.Result{}, nil

}

func (r *DiscoverDeviceController) startDiscoverDeviceDaemonset() error {

	setting, err := r.context.Clientset.CoreV1().ConfigMaps(r.opConfig.OperatorNamespace).Get(r.opManagerContext, operator.OperatorSettingConfigMapName, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			logger.Debug("operator's configmap resource not found. will use default value or env var.")
			r.opConfig.Parameters = make(map[string]string)
		} else {
			// Error reading the object - requeue the request.
			return errors.Wrap(err, "failed to get operator's configmap")
		}
	} else {
		r.opConfig.Parameters = setting.Data
	}

	ownerRef, err := k8sutil.GetDeploymentOwnerReference(r.opManagerContext, r.context.Clientset, os.Getenv(k8sutil.PodNameEnvVar), r.opConfig.OperatorNamespace)
	if err != nil {
		logger.Warningf("could not find deployment owner reference to assign to discover daemonset. %v", err)
	}
	if ownerRef != nil {
		blockOwnerDeletion := false
		ownerRef.BlockOwnerDeletion = &blockOwnerDeletion
	}

	ownerInfo := k8sutil.NewOwnerInfoWithOwnerRef(ownerRef, r.opConfig.OperatorNamespace)

	daemon := getDaemonset(operator.DiscoverAppName, r.opConfig.Image, false, true)

	tolerations := csi.GetToleration(r.opConfig.Parameters, discoverDeviceTolerationsEnv, []corev1.Toleration{})
	nodeAffinity := csi.GetNodeAffinity(r.opConfig.Parameters, discoverDeviceNodeAffinityEnv, &corev1.NodeAffinity{})
	csi.ApplyToPodSpec(&daemon.Spec.Template.Spec, nodeAffinity, tolerations)
	csi.ApplyResourcesToContainers(r.opConfig.Parameters, discoverDeviceResource, &daemon.Spec.Template.Spec)
	err = ownerInfo.SetControllerReference(daemon)
	if err != nil {
		return errors.Wrapf(err, "failed to set owner reference to raw device plugin daemonset %q", daemon.Name)
	}
	err = k8sutil.CreateDaemonSet(r.opManagerContext, daemon.Name, r.opConfig.OperatorNamespace, r.context.Clientset, daemon)
	if err != nil {
		return errors.Wrapf(err, "failed to start discover device daemonset %q", daemon.Name)
	}
	return nil
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	logger.Infof("%s successfully started", controllerName)

	// Watch for ConfigMap (operator config)
	err = c.Watch(&source.Kind{
		Type: &corev1.ConfigMap{TypeMeta: metav1.TypeMeta{Kind: "ConfigMap", APIVersion: v1.SchemeGroupVersion.String()}}}, &handler.EnqueueRequestForObject{}, predicateController())
	if err != nil {
		return err
	}

	return nil
}

func getDaemonset(appName string, image string, useLoop bool, enableRawDevice bool) *v1.DaemonSet {

	var volumes []corev1.Volume
	devVolume := corev1.Volume{Name: "devices", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/dev"}}}
	volumes = append(volumes, devVolume)
	udevVolume := corev1.Volume{Name: "udev", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/run/udev"}}}
	volumes = append(volumes, udevVolume)
	sysVolume := corev1.Volume{Name: "sys", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/sys"}}}
	volumes = append(volumes, sysVolume)

	volumeMount := []corev1.VolumeMount{
		{Name: "devices", MountPath: "/dev", ReadOnly: false},
		{Name: "udev", MountPath: "/run/udev", ReadOnly: true},
		{Name: "sys", MountPath: "/sys", ReadOnly: true},
	}

	privileged := true
	runAsUser := int64(0)
	command := []string{"/topolvm", "discover"}

	resourceRequirements := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(topolvm.TopolvmDiscoverDeviceCPULimit),
			corev1.ResourceMemory: resource.MustParse(topolvm.TopolvmDiscoverDeviceMemLimit),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(topolvm.TopolvmDiscoverDeviceCPURequest),
			corev1.ResourceMemory: resource.MustParse(topolvm.TopolvmDiscoverDeviceMemRequest),
		},
	}
	env := []corev1.EnvVar{
		k8sutil.NamespaceEnvVar(),
		k8sutil.NodeEnvVar(),
		k8sutil.NameEnvVar(),
	}
	annotate := make(map[string]string)

	if useLoop {
		env = append(env, corev1.EnvVar{Name: topolvm.UseLoopEnv, Value: topolvm.UseLoop})
		annotate[topolvm.LoopAnnotationsKey] = topolvm.LoopAnnotationsVal
	}

	if enableRawDevice {
		env = append(env, corev1.EnvVar{Name: operator.EnableRawDeviceEnv, Value: "true"})
	}

	daemonset := &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        appName,
			Namespace:   topolvm.NameSpace,
			Annotations: annotate,
		},
		Spec: v1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					topolvm.AppAttr: appName,
				},
			},
			UpdateStrategy: v1.DaemonSetUpdateStrategy{
				Type: v1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: appName,
					Labels: map[string]string{
						topolvm.AppAttr: appName,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: topolvm.DiscoverDevicesAccount,
					Containers: []corev1.Container{
						{
							Name:      topolvm.DiscoverContainerName,
							Image:     image,
							Command:   command,
							Resources: resourceRequirements,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
								RunAsUser:  &runAsUser,
							},
							VolumeMounts: volumeMount,
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
	return daemonset
}
