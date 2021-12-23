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
	"github.com/alauda/nativestor/pkg/cluster/topolvm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func testPrepareVg() {

	It("should be created", func() {
		Eventually(func() error {
			By("check prepare job count and status")

			result, stderr, err := kubectl("get", "-n", "nativestor-system", "pod", fmt.Sprintf("--selector=%s=%s", topolvm.AppAttr, topolvm.PrePareVgAppName), "-o=json")

			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}
			var podList corev1.PodList
			err = json.Unmarshal(result, &podList)
			if err != nil {
				return err
			}
			if len(podList.Items) != 3 {
				return fmt.Errorf("the number of topolvm-node deployment is not equal to 3: %d", len(podList.Items))
			}

			for _, dev := range podList.Items {
				if dev.Status.Phase != corev1.PodSucceeded {
					podYaml, _ := yaml.Marshal(&dev)
					fmt.Println(string(podYaml))
					return fmt.Errorf("pod %s phase is %s  should be %s", dev.Name, dev.Status.Phase, corev1.PodSucceeded)
				}
			}
			return nil
		}).Should(Succeed())

	})

}
