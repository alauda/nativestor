package k8sutil

import (
	"context"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateCSIDriver(ctx context.Context, clientset kubernetes.Interface, csiDriver *storagev1.CSIDriver) error {
	_, err := clientset.StorageV1().CSIDrivers().Create(ctx, csiDriver, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil
		}
	}
	return err
}

func DeleteCSIDriver(ctx context.Context, clientset kubernetes.Interface, name string) error {

	err := clientset.StorageV1().CSIDrivers().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
	}
	return err
}
