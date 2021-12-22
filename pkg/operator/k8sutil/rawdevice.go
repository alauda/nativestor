package k8sutil

import (
	"context"
	v1 "github.com/alauda/topolvm-operator/apis/rawdevice/v1"
	rawclient "github.com/alauda/topolvm-operator/generated/nativestore/rawdevice/clientset/versioned"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateOrUpdateRawDevice(ctx context.Context, clientset rawclient.Interface, device *v1.RawDevice) (*v1.RawDevice, error) {

	_, err := CreateRawDevice(ctx, clientset, device)
	if k8serrors.IsAlreadyExists(err) {
		newDev, err := clientset.RawdeviceV1().RawDevices().Get(ctx, device.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		newDev.Spec = device.Spec
		dev, err := clientset.RawdeviceV1().RawDevices().Update(ctx, newDev, metav1.UpdateOptions{})
		if err != nil {
			return dev, err
		}
	}

	return nil, err
}

func CreateRawDevice(ctx context.Context, clientset rawclient.Interface, device *v1.RawDevice) (*v1.RawDevice, error) {

	return clientset.RawdeviceV1().RawDevices().Create(ctx, device, metav1.CreateOptions{})

}
