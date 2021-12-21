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

package csidriver

import (
	"context"
	"github.com/alauda/topolvm-operator/pkg/cluster/topolvm"

	"github.com/pkg/errors"
	storagev1 "k8s.io/api/storage/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CheckTopolvmCsiDriverExisting(clientset kubernetes.Interface, ref *metav1.OwnerReference) error {

	_, err := clientset.StorageV1().CSIDrivers().Get(context.TODO(), topolvm.TopolvmCSIDriverName, metav1.GetOptions{})
	if err != nil && !kerrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to detect CsiDriver %s", topolvm.TopolvmCSIDriverName)
	} else if err == nil {
		return nil
	}

	attachRequired := false
	podInfoOnMount := true
	storageCapacity := true
	csiDriver := &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolvm.TopolvmCSIDriverName,
		},
		Spec: storagev1.CSIDriverSpec{
			AttachRequired:       &attachRequired,
			PodInfoOnMount:       &podInfoOnMount,
			StorageCapacity:      &storageCapacity,
			VolumeLifecycleModes: []storagev1.VolumeLifecycleMode{storagev1.VolumeLifecyclePersistent, storagev1.VolumeLifecycleEphemeral},
		},
	}

	_, err = clientset.StorageV1().CSIDrivers().Create(context.TODO(), csiDriver, metav1.CreateOptions{})
	return err
}

func DeleteTopolvmCsiDriver(clientset kubernetes.Interface) error {
	return clientset.StorageV1().CSIDrivers().Delete(context.TODO(), topolvm.TopolvmCSIDriverName, metav1.DeleteOptions{})
}
