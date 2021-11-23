/*
Copyright 2018 The Kubernetes Authors.

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

package converter

// Not being used
import (
	"encoding/json"

	"k8s.io/klog"

	v2 "github.com/alauda/topolvm-operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type TopolvmClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	TopolvmVersion string           `json:"topolvmVersion"`
	DeviceClasses  []v2.NodeDevices `json:"deviceClasses"`
}

func convertExampleCRD(Object *unstructured.Unstructured, toVersion string) (*unstructured.Unstructured, metav1.Status) {
	klog.V(2).Info("converting crd")

	convertedObject := Object.DeepCopy()
	fromVersion := Object.GetAPIVersion()

	if toVersion == fromVersion {
		return nil, statusErrorWithMessage("conversion from a version to itself should not call the webhook: %s", toVersion)
	}

	switch Object.GetAPIVersion() {
	case "topolvm.cybozu.com/v1":
		switch toVersion {
		case "topolvm.cybozu.com/v2":
			conf, ok, _ := unstructured.NestedMap(convertedObject.Object, "spec")
			if ok {

				data, err := json.Marshal(conf)
				if err != nil {
					return nil, statusErrorWithMessage(err.Error())
				}

				devices := TopolvmClusterSpec{}
				err = json.Unmarshal(data, &devices)
				if err != nil {
					return nil, statusErrorWithMessage(err.Error())
				}

				for index1, node := range devices.DeviceClasses {
					for index2, devs := range node.DeviceClasses {
						for index3 := range devs.Device {
							devices.DeviceClasses[index1].DeviceClasses[index2].Device[index3].Type = "disk"
						}
					}
				}

				var result map[string]interface{}

				devs, err := json.Marshal(devices)
				if err != nil {
					return nil, statusErrorWithMessage(err.Error())
				}
				if err := json.Unmarshal(devs, &result); err != nil {
					return nil, statusErrorWithMessage(err.Error())
				}

				delete(convertedObject.Object, "spec")
				unstructured.SetNestedField(convertedObject.Object, devices.TopolvmVersion, "spec", "topolvmVersion")
				unstructured.SetNestedField(convertedObject.Object, result["deviceClasses"], "spec", "storage", "deviceClasses")
				unstructured.SetNestedField(convertedObject.Object, false, "spec", "storage", "useAllNodes")
			}

		default:
			return nil, statusErrorWithMessage("unexpected conversion version %q", toVersion)
		}
	case "topolvm.cybozu.com/v2":
		return convertedObject, statusSucceed()
	default:
		return nil, statusErrorWithMessage("unexpected conversion version %q", fromVersion)
	}
	return convertedObject, statusSucceed()
}
