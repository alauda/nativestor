package csi

const (
	DefaultRegistrarImage   string = "k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.3.0"
	DefaultProvisionerImage string = "k8s.gcr.io/sig-storage/csi-provisioner:v3.0.0"
	DefaultLivenessImage    string = "k8s.gcr.io/sig-storage/livenessprobe:v2.4.0"
	DefaultResizerImage     string = "k8s.gcr.io/sig-storage/csi-resizer:v1.3.0"
	DefaultSnapshotterImage string = "k8s.gcr.io/sig-storage/csi-snapshotter:v4.2.0"
	DefaultAttachment       string = "k8s.gcr.io/sig-storage/csi-attacher:v3.3.0"
	DefaultKubeletDir       string = "/var/lib/kubelet"
)

type Param struct {
	RawDeviceImage               string
	RegistrarImage               string
	ProvisionerImage             string
	AttacherImage                string
	SnapshotterImage             string
	LivenessImage                string
	ResizerImage                 string
	DriverNamePrefix             string
	KubeletDirPath               string
	LogLevel                     uint8
	PluginPriorityClassName      string
	ProvisionerPriorityClassName string
	ProvisionerReplicas          int32
	TopolvmImage                 string
}

type TemplateParam struct {
	Param
	// non-global template only parameters
	Namespace string
}
