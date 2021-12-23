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
	"k8s.io/api/storage/v1alpha1"
	"strings"
)

func testScheduler() {

	testNamespacePrefix := "e2etest-"
	var ns string
	var cc CleanupContext

	BeforeEach(func() {
		cc = commonBeforeEach()

		ns = testNamespacePrefix + randomString(10)
		createNamespace(ns)
	})

	AfterEach(func() {
		// When a test fails, I want to investigate the cause. So please don't remove the namespace!
		if !CurrentGinkgoTestDescription().Failed {
			kubectl("delete", "namespaces/"+ns)
		}

		commonAfterEach(cc)
	})

	It("should be deployed topolvm-scheduler pod", func() {

		pvcYAMLTemplate := `kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: pvc-%s
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: %s
`

		podYAMLTemplate := `apiVersion: v1
kind: Pod
metadata:
  name: %s
  labels:
    app.kubernetes.io/name: %s
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:20.04
      command: ["/usr/local/bin/pause"]
      volumeMounts:
        - mountPath: /test1
          name: my-volume
  volumes:
    - name: my-volume
      persistentVolumeClaim:
        claimName: %s
`

		storageClassPodMap := map[string]string{
			"topolvm-provisioner1": "test-pod1",
			"topolvm-provisioner2": "test-pod2",
			"topolvm-provisioner3": "test-pod3",
		}

		podNodeMap := map[string]string{
			"test-pod1": "topolvm-e2e-worker",
			"test-pod2": "topolvm-e2e-worker2",
			"test-pod3": "topolvm-e2e-worker3",
		}

		Eventually(func() error {
			By("checking csi storage capacity num")
			var csiStorageCapacitiesTemp v1alpha1.CSIStorageCapacityList
			stdout, stderr, err := kubectl("get", "-n", "nativestor-system", "csistoragecapacities", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, stdout, stderr)
			}
			err = json.Unmarshal(stdout, &csiStorageCapacitiesTemp)
			if err != nil {
				return err
			}
			if len(csiStorageCapacitiesTemp.Items) < 3 {
				return fmt.Errorf("csi storagecapacity num:%d should more than %d", len(csiStorageCapacitiesTemp.Items), 3)
			}
			return nil
		}).Should(Succeed())

		for key, val := range storageClassPodMap {

			By("create pvc" + key)
			pvc := fmt.Sprintf(pvcYAMLTemplate, key, key)

			_, _, err := kubectlWithInput([]byte(pvc), "-n", ns, "apply", "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

			By("create " + val)
			pod1 := fmt.Sprintf(podYAMLTemplate, val, val, "pvc-"+key)
			_, _, err = kubectlWithInput([]byte(pod1), "-n", ns, "apply", "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())

		}

		for key, val := range storageClassPodMap {

			By("confirming that the pvc is bound")
			Eventually(func() error {
				stdout, stderr, err := kubectl("-n", ns, "get", "pvc", "pvc-"+key, "-o=template", "--template={{.status.phase}}")
				if err != nil {
					return fmt.Errorf("failed to get pvc. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				}
				phase := strings.TrimSpace(string(stdout))
				if phase != "Bound" {
					return fmt.Errorf("pvc %s is not bind", "pvc-"+key)
				}
				return nil
			}).Should(Succeed())

			By("confirming that the pod is schduler to specific node")
			Eventually(func() error {
				stdout, stderr, err := kubectl("-n", ns, "get", "pod", val, "-o=template", "--template={{.spec.nodeName}}")
				if err != nil {
					return fmt.Errorf("failed to get pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				}
				nodeName := strings.TrimSpace(string(stdout))
				if nodeName != podNodeMap[val] {
					return fmt.Errorf("pod %s is not scheduler to node %s", val, podNodeMap[val])
				}
				return nil
			}).Should(Succeed())

			By("confirming that the pod is running")
			Eventually(func() error {
				stdout, stderr, err := kubectl("-n", ns, "get", "pod", val, "-o=template", "--template={{.status.phase}}")
				if err != nil {
					return fmt.Errorf("failed to get pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
				}
				phase := strings.TrimSpace(string(stdout))
				if phase != "Running" {
					return fmt.Errorf("pod %s not running may be mount failed", val)
				}
				return nil
			}).Should(Succeed())
		}
	})

}
