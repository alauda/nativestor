/*
Copyright 2016 The Rook Authors. All rights reserved.

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
	"github.com/alauda/nativestor/pkg/cluster/topolvm"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
)

// NamespaceEnvVar namespace env var
func NamespaceEnvVar() v1.EnvVar {
	return v1.EnvVar{Name: PodNamespaceEnvVar, ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}}
}

// NameEnvVar pod name env var
func NameEnvVar() v1.EnvVar {
	return v1.EnvVar{Name: PodNameEnvVar, ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.name"}}}
}

// NodeEnvVar node env var
func NodeEnvVar() v1.EnvVar {
	return v1.EnvVar{Name: NodeNameEnvVar, ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}}
}

// GetRunningPod reads the name and namespace of a pod from the
// environment, and returns the pod (if it exists).
func GetRunningPod(clientset kubernetes.Interface) (*v1.Pod, error) {

	podName := os.Getenv(topolvm.PodNameEnv)
	if podName == "" {
		return nil, fmt.Errorf("cannot detect the pod name. Please provide it using the downward API in the manifest file")
	}
	podNamespace := os.Getenv(topolvm.PodNameSpaceEnv)
	if podName == "" {
		return nil, fmt.Errorf("cannot detect the pod namespace. Please provide it using the downward API in the manifest file")
	}

	pod, err := clientset.CoreV1().Pods(podNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return pod, nil
}

// GetContainerImage returns the container image
// matching the given name for a pod. If the pod
// only has a single container, the name argument
// is ignored.
func GetContainerImage(pod *v1.Pod, name string) (string, error) {
	return GetSpecContainerImage(pod.Spec, name, false)
}

// GetSpecContainerImage returns the container image
// for a podspec, given a container name. The name is
// ignored if the podspec has a single container, in
// which case the image for that container is returned.
func GetSpecContainerImage(spec v1.PodSpec, name string, initContainer bool) (string, error) {
	containers := spec.Containers
	if initContainer {
		containers = spec.InitContainers
	}
	image, err := GetMatchingContainer(containers, name)
	if err != nil {
		return "", err
	}
	return image.Image, nil
}

// GetMatchingContainer takes a list of containers and a name,
// and returns the first container in the list matching the
// name. If the list contains a single container it is always
// returned, even if the name does not match.
func GetMatchingContainer(containers []v1.Container, name string) (v1.Container, error) {
	var result *v1.Container
	if len(containers) == 1 {
		// if there is only one pod, use its image rather than require a set container name
		result = &containers[0]
	} else {
		// if there are multiple pods, we require the container to have the expected name
		for _, container := range containers {
			if container.Name == name {
				localcontainer := container
				result = &localcontainer
				break
			}
		}
	}

	if result == nil {
		return v1.Container{}, fmt.Errorf("failed to find image for container %s", name)
	}

	return *result, nil
}
