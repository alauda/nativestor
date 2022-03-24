package controller

import (
	"context"
	"fmt"
	topolvmv2 "github.com/alauda/nativestor/apis/topolvm/v2"
	"github.com/alauda/nativestor/pkg/cluster/topolvm"
	"github.com/alauda/nativestor/pkg/operator/k8sutil"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
	"time"
)

const (
	indexJobMajorNumber = "1"
	// from k8s v1.21 job spec can have a field name 'completionMode' which is
	// in alpha and in v1.22 it's in beta
	indexJobMinorNumber = "22"
)

func (r *TopolvmController) startCleanUpJobs(cluster *topolvmv2.TopolvmCluster) {

	ownerRef, err := k8sutil.GetDeploymentOwnerReference(r.opManagerContext, r.context.Clientset, os.Getenv(k8sutil.PodNameEnvVar), r.opConfig.OperatorNamespace)
	if err != nil {
		logger.Warningf("could not find deployment owner reference to assign to csi drivers. %v", err)
	}
	if ownerRef != nil {
		blockOwnerDeletion := false
		ownerRef.BlockOwnerDeletion = &blockOwnerDeletion
	}

	ownerInfo := k8sutil.NewOwnerInfoWithOwnerRef(ownerRef, r.opConfig.OperatorNamespace)

	var cleanjobs []*batch.Job
	if cluster.Spec.UseAllNodes {
		nodes, err := r.context.Clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logger.Errorf("list node failed err %v", err)
		}
		for _, ele := range nodes.Items {
			job, err := makeJob(r.context.Clientset, ele.Name, r.opConfig.Image)
			if err != nil {
				logger.Errorf("make job failed err %v", err)
			}
			ownerInfo.SetOwnerReference(job)
			err = k8sutil.RunReplaceableJob(r.context.Clientset, job, true)
			if err != nil {
				logger.Errorf("run replaceable job failed err %v", err)
			} else {
				cleanjobs = append(cleanjobs, job)
			}
		}
	}

	if cluster.Spec.Storage.DeviceClasses != nil {
		for _, ele := range cluster.Spec.Storage.DeviceClasses {
			job, err := makeJob(r.context.Clientset, ele.NodeName, r.opConfig.Image)
			if err != nil {
				logger.Errorf("make job failed err %v", err)
			}
			ownerInfo.SetOwnerReference(job)
			err = k8sutil.RunReplaceableJob(r.context.Clientset, job, true)
			if err != nil {
				logger.Errorf("run replaceable job failed err %v", err)
			} else {
				cleanjobs = append(cleanjobs, job)
			}
		}
	}

	for _, j := range cleanjobs {
		err = k8sutil.WaitForJobCompletion(r.context.Clientset, j, 15*time.Minute)
		if err != nil {
			logger.Errorf("wait for job completion failed err %v", err)
		}
	}

}

func makeJob(clientset kubernetes.Interface, nodeName string, image string) (*batch.Job, error) {

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
			Name:      truncateNodeNameForIndexJob(topolvm.CleanDeviceJobFmt, nodeName),
			Namespace: topolvm.NameSpace,
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

	command := []string{"/topolvm", "clean"}

	privileged := true
	runAsUser := int64(0)

	podSpec := v1.PodSpec{
		ServiceAccountName: topolvm.CleanUpServiceAccount,
		Containers: []v1.Container{
			{
				Name:    "clean",
				Image:   image,
				Command: command,
				SecurityContext: &v1.SecurityContext{
					Privileged: &privileged,
					RunAsUser:  &runAsUser,
				},
				VolumeMounts: volumeMount,
				Env: []v1.EnvVar{
					{Name: topolvm.NodeNameEnv, Value: nodeName},
					{Name: topolvm.PodNameSpaceEnv, Value: topolvm.NameSpace},
					{Name: topolvm.LogLevelEnv, Value: topolvm.CleanUpJobLogLevel},
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
		Name:        "clean",
		Annotations: map[string]string{},
	}
	return &v1.PodTemplateSpec{
		ObjectMeta: podMeta,
		Spec:       podSpec,
	}, nil

}

func truncateNodeNameForIndexJob(format, nodeName string) string {
	hashed := k8sutil.Hash(nodeName)
	logger.Infof("nodeName %s will be %s", nodeName, hashed)
	return fmt.Sprintf(format, hashed)
}
