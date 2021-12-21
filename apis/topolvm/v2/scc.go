package v2

import (
	"fmt"

	secv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewSecurityContextConstraints returns a new SecurityContextConstraints for topolvm to run on
// OpenShift.
func NewSecurityContextConstraints(name, namespace string) *secv1.SecurityContextConstraints {
	return &secv1.SecurityContextConstraints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "security.openshift.io/v1",
			Kind:       "SecurityContextConstraints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		AllowPrivilegedContainer: true,
		AllowHostDirVolumePlugin: true,
		ReadOnlyRootFilesystem:   false,
		AllowHostPID:             true,
		AllowHostIPC:             true,
		AllowHostNetwork:         false,
		AllowHostPorts:           false,
		RequiredDropCapabilities: []corev1.Capability{},
		DefaultAddCapabilities:   []corev1.Capability{},
		RunAsUser: secv1.RunAsUserStrategyOptions{
			Type: secv1.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: secv1.SELinuxContextStrategyOptions{
			Type: secv1.SELinuxStrategyMustRunAs,
		},
		FSGroup: secv1.FSGroupStrategyOptions{
			Type: secv1.FSGroupStrategyMustRunAs,
		},
		SupplementalGroups: secv1.SupplementalGroupsStrategyOptions{
			Type: secv1.SupplementalGroupsStrategyRunAsAny,
		},
		Volumes: []secv1.FSType{
			secv1.FSTypeConfigMap,
			secv1.FSTypeEmptyDir,
			secv1.FSTypeHostPath,
			secv1.FSTypeSecret,
		},
		Users: []string{
			fmt.Sprintf("system:serviceaccount:%s:topolvm-node", namespace),
			fmt.Sprintf("system:serviceaccount:%s:topolvm-discover", namespace),
			fmt.Sprintf("system:serviceaccount:%s:topolvm-preparevg", namespace),
		},
	}
}
