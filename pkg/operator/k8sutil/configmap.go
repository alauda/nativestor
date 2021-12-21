/*
Copyright 2019 The Rook Authors. All rights reserved.

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
	"encoding/json"
	"fmt"
	"github.com/alauda/topolvm-operator/pkg/cluster/topolvm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"os"
	"time"
)

func CreateReplaceableConfigmap(clientset kubernetes.Interface, configmap *corev1.ConfigMap) error {

	ctx := context.Background()
	existingCm, err := clientset.CoreV1().ConfigMaps(configmap.Namespace).Get(ctx, configmap.Name, metav1.GetOptions{})

	if err != nil && !errors.IsNotFound(err) {
		logger.Warningf("failed to detect configmap %s. %+v", configmap.Name, err)
	} else if err == nil {
		// delete the configmap that already exists from a previous run
		logger.Infof("Removing previous cm %s to start a new one", configmap.Name)

		err := DeleteConfigMap(clientset, existingCm.Name, existingCm.Namespace, &DeleteOptions{MustDelete: true})
		if err != nil {
			logger.Warningf("failed to remove configmap %s. %+v", configmap.Name, err)
		}
	}
	_, err = clientset.CoreV1().ConfigMaps(configmap.Namespace).Create(ctx, configmap, metav1.CreateOptions{})
	return err

}

func CreateOrPatchConfigmap(clientset kubernetes.Interface, configmap *corev1.ConfigMap) error {

	ctx := context.Background()
	existingCm, err := clientset.CoreV1().ConfigMaps(configmap.Namespace).Get(ctx, configmap.Name, metav1.GetOptions{})

	if err != nil && !errors.IsNotFound(err) {
		logger.Warningf("failed to detect configmap %s. %+v", configmap.Name, err)
	} else if err == nil {
		// delete the configmap that already exists from a previous run
		logger.Infof("patching previous cm %s", configmap.Name)

		return PatchConfigMap(clientset, existingCm.Namespace, existingCm, configmap)
	}
	_, err = clientset.CoreV1().ConfigMaps(configmap.Namespace).Create(ctx, configmap, metav1.CreateOptions{})
	return err

}

func DeleteConfigMap(clientset kubernetes.Interface, cmName, namespace string, opts *DeleteOptions) error {
	ctx := context.Background()
	k8sOpts := BaseKubernetesDeleteOptions()
	d := func() error { return clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, cmName, *k8sOpts) }
	verify := func() error {
		_, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, cmName, metav1.GetOptions{})
		return err
	}
	resource := fmt.Sprintf("ConfigMap %s", cmName)
	defaultWaitOptions := &WaitOptions{RetryCount: 20, RetryInterval: 2 * time.Second}
	return DeleteResource(d, verify, resource, opts, defaultWaitOptions)
}

func PatchConfigMap(clientset kubernetes.Interface, namespace string, oldConfigMap, newConfigMap *corev1.ConfigMap) error {
	newJSON, err := json.Marshal(*newConfigMap)
	if err != nil {
		return err
	}
	oldJSON, err := json.Marshal(*oldConfigMap)
	if err != nil {
		return err
	}
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldJSON, newJSON, corev1.ConfigMap{})
	if err != nil {
		return err
	}
	_, err = clientset.CoreV1().ConfigMaps(namespace).Patch(context.TODO(), oldConfigMap.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		logger.Infof("failed to patch configmap %s: %v", oldConfigMap.Name, err)
		return err
	}
	return nil
}

// GetOperatorSetting gets the operator setting from ConfigMap or Env Var
// returns defaultValue if setting is not found
func GetOperatorSetting(clientset kubernetes.Interface, configMapName, settingName, defaultValue string) (string, error) {
	// config must be in operator pod namespace
	ctx := context.TODO()
	cm, err := clientset.CoreV1().ConfigMaps(topolvm.NameSpace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			if settingValue, ok := os.LookupEnv(settingName); ok {
				logger.Infof("%s=%q (env var)", settingName, settingValue)
				return settingValue, nil
			}
			logger.Infof("%s=%q (default)", settingName, defaultValue)
			return defaultValue, nil
		}
		return defaultValue, fmt.Errorf("error reading ConfigMap %q. %v", configMapName, err)
	}
	if settingValue, ok := cm.Data[settingName]; ok {
		logger.Infof("%s=%q (configmap)", settingName, settingValue)
		return settingValue, nil
	} else if settingValue, ok := os.LookupEnv(settingName); ok {
		logger.Infof("%s=%q (env var)", settingName, settingValue)
		return settingValue, nil
	}
	logger.Infof("%s=%q (default)", settingName, defaultValue)
	return defaultValue, nil
}

func GetValue(data map[string]string, settingName, defaultValue string) string {
	if settingValue, ok := data[settingName]; ok {
		logger.Infof("%s=%q (configmap)", settingName, settingValue)
		return settingValue
	} else if settingValue, ok := os.LookupEnv(settingName); ok {
		logger.Infof("%s=%q (env var)", settingName, settingValue)
		return settingValue
	}
	logger.Infof("%s=%q (default)", settingName, defaultValue)
	return defaultValue
}
