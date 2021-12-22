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

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/storage/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	"os/exec"
	"strings"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/topolvm/topolvm"
)

type CleanupContext struct {
	NodeCapacityAnnotations map[string]map[string]string
}

func execAtLocal(cmd string, input []byte, args ...string) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	command := exec.Command(cmd, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr

	if len(input) != 0 {
		command.Stdin = bytes.NewReader(input)
	}

	err := command.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func kubectl(args ...string) ([]byte, []byte, error) {
	return execAtLocal("kubectl", nil, args...)
}

func kubectlWithInput(input []byte, args ...string) ([]byte, []byte, error) {
	return execAtLocal("kubectl", input, args...)
}

func commonBeforeEach() CleanupContext {
	var cc CleanupContext
	var err error

	cc.NodeCapacityAnnotations, err = getNodeAnnotationMapWithPrefix(topolvm.CapacityKeyPrefix)
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred())

	return cc
}

func commonAfterEach(cc CleanupContext) {
	if !CurrentGinkgoTestDescription().Failed {
		EventuallyWithOffset(1, func() error {
			stdout, stderr, err := kubectl("get", "node", "-o", "json")
			if err != nil {
				return fmt.Errorf("stdout=%s, stderr=%s", stdout, stderr)
			}

			capacitiesAfter, err := getNodeAnnotationMapWithPrefix(topolvm.CapacityKeyPrefix)
			if err != nil {
				return err
			}
			if diff := cmp.Diff(cc.NodeCapacityAnnotations, capacitiesAfter); diff != "" {
				return fmt.Errorf("capacities on nodes should be same before and after the test: diff=%q", diff)
			}
			return nil
		}).Should(Succeed())
	}
}

func getNodeAnnotationMapWithPrefix(prefix string) (map[string]map[string]string, error) {
	stdout, stderr, err := kubectl("get", "node", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("stdout=%sr stderr=%s, err=%v", stdout, stderr, err)
	}

	var nodeList corev1.NodeList
	err = json.Unmarshal(stdout, &nodeList)
	if err != nil {
		return nil, err
	}

	capacities := make(map[string]map[string]string)
	for _, node := range nodeList.Items {
		if node.Name == "topolvm-e2e-control-plane" || node.Name == "topolvm-e2e-worker" {
			continue
		}

		capacities[node.Name] = make(map[string]string)
		for k, v := range node.Annotations {
			if !strings.HasPrefix(k, prefix) {
				continue
			}
			capacities[node.Name][k] = v
		}
	}
	return capacities, nil
}

func getCSICapacity() (map[string]*resource.Quantity, error) {
	stdout, stderr, err := kubectl("get", "-n", "topolvm-system", "csistoragecapacities", "-o=json")
	if err != nil {
		return nil, fmt.Errorf("stdout=%sr stderr=%s, err=%v", stdout, stderr, err)
	}
	var csiStorageCapacities v1alpha1.CSIStorageCapacityList
	err = json.Unmarshal(stdout, &csiStorageCapacities)
	if err != nil {
		return nil, fmt.Errorf("unmashal CSIStorageCapacityList failed err %v", err)
	}
	capacities := make(map[string]*resource.Quantity)
	for _, val := range csiStorageCapacities.Items {
		capacities[val.Name] = val.Capacity
	}
	return capacities, nil

}

func waitCreatingDefaultSA(ns string) error {
	stdout, stderr, err := kubectl("get", "sa", "-n", ns, "default")
	if err != nil {
		return fmt.Errorf("default sa is not found. stdout=%s, stderr=%s, err=%v", stdout, stderr, err)
	}
	return nil
}

func checkPodReady(pod *corev1.Pod) error {
	podReady := false
	for _, cond := range pod.Status.Conditions {
		fmt.Fprintln(GinkgoWriter, cond)
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			podReady = true
			break
		}
	}
	if !podReady {
		return errors.New("pod is not yet ready")
	}

	return nil
}
