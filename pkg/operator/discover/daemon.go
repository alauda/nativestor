package discover

import (
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/operator/k8sutil"
	"github.com/pkg/errors"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func MakeDiscoverDevicesDaemonset(clientset kubernetes.Interface, appName string, image string, useLoop bool, reference *metav1.OwnerReference) error {

	daemon := getDaemonset(appName, image, useLoop, reference)
	if err := k8sutil.CreateDaemonSet(appName, cluster.NameSpace, clientset, daemon); err != nil {
		return errors.Wrapf(err, "create daemonset  %s failed", appName)
	}
	return nil
}

func getDaemonset(appName string, image string, useLoop bool, ref *metav1.OwnerReference) *v1.DaemonSet {

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

	var loop corev1.EnvVar
	if useLoop {
		loop = corev1.EnvVar{Name: cluster.UseLoopEnv, Value: cluster.UseLoop}
	} else {
		loop = corev1.EnvVar{Name: cluster.UseLoopEnv, Value: "0"}
	}

	daemonset := &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            appName,
			Namespace:       cluster.NameSpace,
			OwnerReferences: []metav1.OwnerReference{*ref},
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
					ServiceAccountName: cluster.DiscoverDevicesAccount,
					Containers: []corev1.Container{
						{
							Name:    cluster.DiscoverContainerName,
							Image:   image,
							Command: command,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
								RunAsUser:  &runAsUser,
							},
							VolumeMounts: volumeMount,
							Env: []corev1.EnvVar{
								k8sutil.NamespaceEnvVar(),
								k8sutil.NodeEnvVar(),
								k8sutil.NameEnvVar(),
								loop,
							},
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
