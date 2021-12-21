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
)

// CreateDaemonSet creates
func CreateDaemonSet(ctx context.Context, name, namespace string, clientset kubernetes.Interface, ds *apps.DaemonSet) error {
	_, err := clientset.AppsV1().DaemonSets(namespace).Create(ctx, ds, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			_, err = clientset.AppsV1().DaemonSets(namespace).Update(ctx, ds, metav1.UpdateOptions{})
		}
		if err != nil {
			return fmt.Errorf("failed to start %s daemonset: %+v\n%+v", name, err, ds)
		}
	}
	return err
}

// DeleteDaemonset makes a best effort at deleting a daemonset and its pods, then waits for them to be deleted
func DeleteDaemonset(ctx context.Context, clientset kubernetes.Interface, namespace, name string) error {
	deleteAction := func(options *metav1.DeleteOptions) error {
		return clientset.AppsV1().DaemonSets(namespace).Delete(ctx, name, *options)
	}
	getAction := func() error {
		_, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		return err
	}
	return deleteResourceAndWait(namespace, name, "daemonset", deleteAction, getAction)
}

// GetDaemonsets returns a list of daemonsets names labels matching a given selector
// example of a label selector might be "app=rook-ceph-mon, mon!=b"
// more: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
func GetDaemonsets(clientset kubernetes.Interface, namespace, labelSelector string) (*apps.DaemonSetList, error) {
	listOptions := metav1.ListOptions{LabelSelector: labelSelector}
	ctx := context.TODO()
	daemonsets, err := clientset.AppsV1().DaemonSets(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments with labelSelector %s: %v", labelSelector, err)
	}
	return daemonsets, nil
}

func GetDaemonset(clientset kubernetes.Interface, namespace, name string) (*apps.DaemonSet, error) {
	ctx := context.TODO()
	daemonset, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get daemonset  %s: %v", name, err)
	}
	return daemonset, nil
}
