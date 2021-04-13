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
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/storage/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func testCsiStorageCapacity() {

	It("should be annotationed", func() {
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "nodes", "-o=json")
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
			var nodes corev1.NodeList
			err = json.Unmarshal(stdout, &nodes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(nodes.Items)).To(Equal(4))

			classNameMap := map[string]string{
				"topolvm-e2e-worker":        "hdd1",
				"topolvm-e2e-worker2":       "hdd2",
				"topolvm-e2e-worker3":       "hdd3",
				"topolvm-e2e-control-plane": "",
			}

			for _, node := range nodes.Items {
				className, ok := classNameMap[node.Name]
				if !ok {
					panic(node.Name + " does not exist")
				}
				if len(className) == 0 {
					continue
				}
				By("checking " + node.Name)
				_, ok = node.Annotations[topolvm.CapacityKeyPrefix+className]
				Expect(ok).To(Equal(true), "capacity is not annotated: "+node.Name)
			}

			return nil
		}).Should(Succeed())
	})

	It("should csistoragecapacities create and update", func() {

		stdout, stderr, err := kubectl("get", "-n", "topolvm-system", "csistoragecapacities", "-o=json")
		Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
		var csiStorageCapacities v1alpha1.CSIStorageCapacityList
		err = json.Unmarshal(stdout, &csiStorageCapacities)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(csiStorageCapacities.Items)).To(Equal(3))

		csiStorageCapacitiesMap := map[string]*resource.Quantity{
			"topolvm-provisioner1": nil,
			"topolvm-provisioner2": nil,
			"topolvm-provisioner3": nil,
		}

		var updateTestName string

		for _, s := range csiStorageCapacities.Items {
			_, ok := csiStorageCapacitiesMap[s.StorageClassName]
			if s.StorageClassName == "topolvm-provisioner1" {
				updateTestName = s.Name
			}
			Expect(ok).To(Equal(true), fmt.Sprintf("csiStorageCapacities %s should not has other sc %s", s.Name, s.StorageClassName))
			By("checking " + s.Name)

			csiStorageCapacitiesMap[s.StorageClassName] = s.Capacity
		}

		topolvmClusterYaml := []byte(`apiVersion: topolvm.cybozu.com/v1
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  namespace: topolvm-system
spec:
  topolvmVersion: harbor-b.alauda.cn/acp/topolvm:v3.4-pr-3.15
  deviceClasses:
    - nodeName: "topolvm-e2e-worker"
      classes:
        - className: "hdd1"
          volumeGroup: "node1-myvg1"
          default: true
          devices:
            - name: "/dev/loop0"
            - name: "/dev/loop3"
    - nodeName: "topolvm-e2e-worker2"
      classes:
        - className: "hdd2"
          volumeGroup: "node2-myvg1"
          default: true
          devices:
            - name: "/dev/loop1"
    - nodeName: "topolvm-e2e-worker3"
      classes:
        - className: "hdd3"
          volumeGroup: "node3-myvg1"
          default: true
          devices:
            - name: "/dev/loop2"
`)
		_, _, err = kubectlWithInput(topolvmClusterYaml, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {

			stdout, stderr, err := kubectl("get", "-n", "topolvm-system", "csistoragecapacities", updateTestName, "-o=json")
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)
			var csiStorageCapacities v1alpha1.CSIStorageCapacity
			err = json.Unmarshal(stdout, &csiStorageCapacities)
			Expect(err).ShouldNot(HaveOccurred())

			capacity := csiStorageCapacities.Capacity

			if !capacity.Equal(*csiStorageCapacitiesMap["topolvm-provisioner1"]) {
				return errors.New("capacity should change when vg expand")
			}
			return nil

		}).Should(Succeed())

	})

}
