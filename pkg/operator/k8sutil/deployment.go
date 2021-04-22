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
	apps "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

func CreateDeployment(clientset kubernetes.Interface, name, namespace string, dep *apps.Deployment) error {

	ctx := context.Background()
	_, err := clientset.AppsV1().Deployments(namespace).Create(ctx, dep, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			_, err = clientset.AppsV1().Deployments(namespace).Update(ctx, dep, metav1.UpdateOptions{})
		}
		if err != nil {
			return fmt.Errorf("failed to start %s deployment: %+v\n%+v", name, err, dep)
		}
	}
	return err
}

// DeleteDeployment makes a best effort at deleting a deployment and its pods, then waits for them to be deleted
func DeleteDeployment(clientset kubernetes.Interface, namespace, name string) error {
	ctx := context.TODO()
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
