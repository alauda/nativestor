module github.com/alauda/topolvm-operator

go 1.16

require (
	github.com/banzaicloud/k8s-objectmatcher v1.6.1
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f
	github.com/google/go-cmp v0.5.6
	github.com/google/uuid v1.1.2
	github.com/kubernetes-csi/csi-lib-utils v0.10.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/openshift/api v0.0.0-20211122204231-b094ceff1955
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.52.1
	github.com/prometheus-operator/prometheus-operator/pkg/client v0.52.1
	github.com/prometheus/client_golang v1.11.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/topolvm/topolvm v0.10.2
	golang.org/x/sys v0.0.0-20210831042530-f4d43177bf5e
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.3
	k8s.io/apiextensions-apiserver v0.22.3
	k8s.io/apimachinery v0.22.3
	k8s.io/client-go v0.22.3
	k8s.io/cloud-provider v0.22.3
	k8s.io/klog v1.0.0
	k8s.io/mount-utils v0.21.1
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/controller-runtime v0.10.0
	sigs.k8s.io/yaml v1.2.0
)
