module github.com/alauda/topolvm-operator

go 1.15

require (
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f
	github.com/google/go-cmp v0.5.4
	github.com/google/uuid v1.1.2
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/topolvm/topolvm v0.8.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.20.2
	k8s.io/apiextensions-apiserver v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.2
)

replace (
	k8s.io/api => k8s.io/api v0.20.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.0
	k8s.io/apiserver => k8s.io/apiserver v0.20.0
	k8s.io/client-go => k8s.io/client-go v0.20.0
	k8s.io/code-generator => k8s.io/code-generator v0.20.0
	k8s.io/component-base => k8s.io/component-base v0.20.0
	k8s.io/gengo => k8s.io/gengo v0.0.0-20201113003025-83324d819ded
	k8s.io/klog/v2 => k8s.io/klog/v2 v2.4.0
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd
	k8s.io/mount-utils => k8s.io/mount-utils v0.20.0
	k8s.io/utils => k8s.io/utils v0.0.0-20201110183641-67b214c5f920
)
