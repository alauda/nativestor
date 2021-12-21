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

package k8sutil

import (
	"context"
	"github.com/alauda/topolvm-operator/pkg/cluster/topolvm"
	"github.com/stretchr/testify/assert"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func makeDeployment(deploymentName string, nameSpace string, image string) *apps.Deployment {
	replicas := int32(1)
	container := corev1.Container{
		Name:  "test-container",
		Image: image,
	}

	dep := &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: nameSpace,
		},
		Spec: apps.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					topolvm.AppAttr: deploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: deploymentName,
					Labels: map[string]string{
						topolvm.AppAttr: deploymentName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
				},
			},
		},
	}

	return dep
}

func TestCreateDeployment(t *testing.T) {
	k8s := fake.NewSimpleClientset()
	deploymentName := "test-deployment"
	nameSpace := "test-namespace"
	image := "test-image"

	dep := makeDeployment(deploymentName, nameSpace, image)

	_, err := CreateDeployment(context.TODO(), k8s, dep)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.TODO()

	_, err = k8s.AppsV1().Deployments(nameSpace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	dep.Spec.Template.Spec.Containers[0].Image = "new-image"
	_, err = CreateOrUpdateDeployment(context.TODO(), k8s, dep)
	if err != nil {
		t.Fatal(err)
	}

	newDep, err := k8s.AppsV1().Deployments(nameSpace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "new-image", newDep.Spec.Template.Spec.Containers[0].Image)
}

func TestDeleteDeployment(t *testing.T) {
	k8s := fake.NewSimpleClientset()
	deploymentName := "test-deployment"
	nameSpace := "test-namespace"
	image := "test-image"

	dep := makeDeployment(deploymentName, nameSpace, image)

	ctx := context.TODO()
	_, err := k8s.AppsV1().Deployments(nameSpace).Create(ctx, dep, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	err = DeleteDeployment(ctx, k8s, nameSpace, deploymentName)
	if err != nil {
		t.Fatal(err)
	}

	_, err = k8s.AppsV1().Deployments(nameSpace).Get(ctx, deploymentName, metav1.GetOptions{})

	assert.Equal(t, true, kerrors.IsNotFound(err))

}
