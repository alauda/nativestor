/*
Copyright 2018 The Rook Authors. All rights reserved.

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
	"fmt"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

func CreateDeployment(ctx context.Context, clientset kubernetes.Interface, dep *appsv1.Deployment) (*appsv1.Deployment, error) {
	// Set hash annotation to the newly generated deployment
	err := patch.DefaultAnnotator.SetLastAppliedAnnotation(dep)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set hash annotation on deployment %q", dep.Name)
	}

	return clientset.AppsV1().Deployments(dep.Namespace).Create(ctx, dep, metav1.CreateOptions{})
}

// DeleteDeployment makes a best effort at deleting a deployment and its pods, then waits for them to be deleted
func DeleteDeployment(ctx context.Context, clientset kubernetes.Interface, namespace, name string) error {
	logger.Debugf("removing %s deployment if it exists", name)
	deleteAction := func(options *metav1.DeleteOptions) error {
		return clientset.AppsV1().Deployments(namespace).Delete(ctx, name, *options)
	}
	getAction := func() error {
		_, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		return err
	}
	return deleteResourceAndWait(namespace, name, "deployment", deleteAction, getAction)
}

// deleteResourceAndWait will delete a resource, then wait for it to be purged from the system
func deleteResourceAndWait(namespace, name, resourceType string,
	deleteAction func(*metav1.DeleteOptions) error,
	getAction func() error,
) error {
	var gracePeriod int64
	propagation := metav1.DeletePropagationForeground
	options := &metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod, PropagationPolicy: &propagation}

	// Delete the resource if it exists
	logger.Infof("removing %s %s if it exists", resourceType, name)
	err := deleteAction(options)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete %s. %+v", name, err)
		}
		return nil
	}
	logger.Infof("Removed %s %s", resourceType, name)

	// wait for the resource to be deleted
	sleepTime := 2 * time.Second
	for i := 0; i < 45; i++ {
		// check for the existence of the resource
		err = getAction()
		if err != nil {
			if k8serrors.IsNotFound(err) {
				logger.Infof("confirmed %s does not exist", name)
				return nil
			}
			return fmt.Errorf("failed to get %s. %+v", name, err)
		}

		if i%5 == 0 {
			// occasionally print a message
			logger.Infof("%q still found. waiting...", name)
		}
		time.Sleep(sleepTime)
	}

	return fmt.Errorf("gave up waiting for %s pods to be terminated", name)
}

func CreateOrUpdateDeployment(ctx context.Context, clientset kubernetes.Interface, dep *appsv1.Deployment) (*appsv1.Deployment, error) {
	newDep, err := CreateDeployment(ctx, clientset, dep)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			// annotation was added in CreateDeployment to dep passed by reference
			newDep, err = clientset.AppsV1().Deployments(dep.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
		}
		if err != nil {
			logger.Errorf("CreateOrUpdateDeployment failed deploy %s, err %v", dep.Name, err)
			return nil, errors.Wrapf(err, "failed to create or update deployment %q: %+v", dep.Name, dep)
		}
	}
	return newDep, nil
}

// GetDeploymentOwnerReference returns an OwnerReference to the deployment that is running the given pod name
func GetDeploymentOwnerReference(ctx context.Context, clientset kubernetes.Interface, podName, namespace string) (*metav1.OwnerReference, error) {
	var deploymentRef *metav1.OwnerReference
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "could not find pod %q in namespace %q to find deployment owner reference", podName, namespace)
	}
	for _, podOwner := range pod.OwnerReferences {
		if podOwner.Kind == "ReplicaSet" {
			replicaset, err := clientset.AppsV1().ReplicaSets(namespace).Get(ctx, podOwner.Name, metav1.GetOptions{})
			if err != nil {
				return nil, errors.Wrapf(err, "could not find replicaset %q in namespace %q to find deployment owner reference", podOwner.Name, namespace)
			}
			for _, replicasetOwner := range replicaset.OwnerReferences {
				if replicasetOwner.Kind == "Deployment" {
					localreplicasetOwner := replicasetOwner
					deploymentRef = &localreplicasetOwner
				}
			}
		}
	}
	if deploymentRef == nil {
		return nil, errors.New("could not find owner reference for deployment")
	}
	return deploymentRef, nil
}

func CheckDeploymentIsExisting(ctx context.Context, clientset kubernetes.Interface, deploymentName, namespace string) (bool, error) {
	_, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return false, errors.Wrapf(err, "failed to detect deployment %s", deploymentName)
	} else if err == nil {
		return true, nil
	}
	return false, nil
}
