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

package psp

import (
	"context"
	"github.com/alauda/topolvm-operator/pkg/cluster/topolvm"

	"github.com/pkg/errors"
	"k8s.io/api/policy/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateTopolvmNodePsp(clientset kubernetes.Interface, ref *metav1.OwnerReference) error {

	allowPrivilegeEscalation := true

	topolvmNodePsp := &v1beta1.PodSecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolvm.TopolvmNodePsp,
		},
		Spec: v1beta1.PodSecurityPolicySpec{
			Privileged:               true,
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
			Volumes:                  []v1beta1.FSType{v1beta1.ConfigMap, v1beta1.EmptyDir, v1beta1.Secret, v1beta1.HostPath},
			AllowedHostPaths: []v1beta1.AllowedHostPath{
				{PathPrefix: "/var/lib/kubelet", ReadOnly: false},
				{PathPrefix: "/run/topolvm", ReadOnly: false},
				{PathPrefix: "/etc/topolvm", ReadOnly: false},
			},
			HostNetwork:            true,
			HostPID:                true,
			RunAsUser:              v1beta1.RunAsUserStrategyOptions{Rule: v1beta1.RunAsUserStrategyRunAsAny},
			SELinux:                v1beta1.SELinuxStrategyOptions{Rule: v1beta1.SELinuxStrategyRunAsAny},
			SupplementalGroups:     v1beta1.SupplementalGroupsStrategyOptions{Rule: v1beta1.SupplementalGroupsStrategyRunAsAny},
			FSGroup:                v1beta1.FSGroupStrategyOptions{Rule: v1beta1.FSGroupStrategyRunAsAny},
			ReadOnlyRootFilesystem: true,
		},
	}
	_, err := clientset.PolicyV1beta1().PodSecurityPolicies().Create(context.TODO(), topolvmNodePsp, metav1.CreateOptions{})

	return err
}

func CheckPspExisting(clientset kubernetes.Interface, name string) (bool, error) {

	_, err := clientset.PolicyV1beta1().PodSecurityPolicies().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return false, errors.Wrapf(err, "failed to detect PodSecurityPolicies %s", name)
	} else if err == nil {
		return true, nil
	}
	return false, nil
}

func CreateTopolvmPrepareVgPsp(clientset kubernetes.Interface, ref *metav1.OwnerReference) error {

	allowPrivilegeEscalation := true

	topolvmPrepareVgPsp := &v1beta1.PodSecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolvm.TopolvmPrepareVgPsp,
		},
		Spec: v1beta1.PodSecurityPolicySpec{
			Privileged:               true,
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
			Volumes:                  []v1beta1.FSType{v1beta1.ConfigMap, v1beta1.EmptyDir, v1beta1.Secret, v1beta1.HostPath},
			AllowedHostPaths: []v1beta1.AllowedHostPath{
				{PathPrefix: "/dev", ReadOnly: false},
				{PathPrefix: "/run/udev", ReadOnly: false},
				{PathPrefix: "/sys", ReadOnly: false},
			},
			HostNetwork:            true,
			HostPID:                true,
			HostIPC:                true,
			RunAsUser:              v1beta1.RunAsUserStrategyOptions{Rule: v1beta1.RunAsUserStrategyRunAsAny},
			SELinux:                v1beta1.SELinuxStrategyOptions{Rule: v1beta1.SELinuxStrategyRunAsAny},
			SupplementalGroups:     v1beta1.SupplementalGroupsStrategyOptions{Rule: v1beta1.SupplementalGroupsStrategyRunAsAny},
			FSGroup:                v1beta1.FSGroupStrategyOptions{Rule: v1beta1.FSGroupStrategyRunAsAny},
			ReadOnlyRootFilesystem: true,
		},
	}

	_, err := clientset.PolicyV1beta1().PodSecurityPolicies().Create(context.TODO(), topolvmPrepareVgPsp, metav1.CreateOptions{})

	return err
}
