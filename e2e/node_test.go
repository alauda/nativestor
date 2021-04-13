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
	"github.com/topolvm/topolvm"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"topolvm-operator/pkg/cluster"
)

func testNode() {

	It("should be created", func() {
		Eventually(func() error {

			By("check lvmd configmap count")
			result, stderr, err := kubectl("get", "-n=topolvm-system", "configmap", fmt.Sprintf("--selector=%s=%s", cluster.LvmdConfigMapLabelKey, cluster.LvmdConfigMapLabelValue), "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}
			var cmList corev1.ConfigMapList
			err = json.Unmarshal(result, &cmList)
			if err != nil {
				return err
			}
			if len(cmList.Items) != 3 {
				return fmt.Errorf("the number of topolvm-node deployment is not equal to 3: %d", len(cmList.Items))
			}

			classNameMap := map[string]string{
				"topolvm-e2e-worker":        "hdd1",
				"topolvm-e2e-worker2":       "hdd2",
				"topolvm-e2e-worker3":       "hdd3",
				"topolvm-e2e-control-plane": "",
			}

			By("checking lvmd classname")
			for _, cm := range cmList.Items {
				lmvdConf := &cluster.LmvdConf{}
				err = yaml.Unmarshal([]byte(cm.Data[cluster.LvmdConfigMapKey]), lmvdConf)
				if err != nil {
					return err
				}
				nodename := cm.Annotations[cluster.LvmdAnnotationsNodeKey]
				if lmvdConf.DeviceClasses[0].Name != classNameMap[nodename] {
					return fmt.Errorf("cm %s lvmd class name %s not equal %s", cm.Name, lmvdConf.DeviceClasses[0].Name, classNameMap[nodename])
				}
			}
			return nil
		}).Should(Succeed())
	})

	It("should be deployed", func() {
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=topolvm-system", "deployment", "--selector=app.kubernetes.io/name=topolvm-node", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}
			var deploymentList appsv1.DeploymentList
			err = json.Unmarshal(result, &deploymentList)
			if err != nil {
				return err
			}
			if len(deploymentList.Items) != 3 {
				return fmt.Errorf("the number of topolvm-node deployment is not equal to 3: %d", len(deploymentList.Items))
			}
			return nil
		}).Should(Succeed())
	})

	It("should annotate capacity to node", func() {
		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "nodes", "-o=json")
			Expect(err).ShouldNot(HaveOccurred(), "stdout=%s, stderr=%s", stdout, stderr)

			var nodes corev1.NodeList
			err = json.Unmarshal(stdout, &nodes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(len(nodes.Items)).To(Equal(4))

			vgNameMap := map[string]string{
				"topolvm-e2e-worker":        "node1-myvg1",
				"topolvm-e2e-worker2":       "node2-myvg1",
				"topolvm-e2e-worker3":       "node3-myvg1",
				"topolvm-e2e-control-plane": "",
			}

			classNameMap := map[string]string{
				"topolvm-e2e-worker":        "hdd1",
				"topolvm-e2e-worker2":       "hdd2",
				"topolvm-e2e-worker3":       "hdd3",
				"topolvm-e2e-control-plane": "",
			}

			for _, node := range nodes.Items {
				vgName, ok := vgNameMap[node.Name]
				if !ok {
					panic(node.Name + " does not exist")
				}

				if len(vgName) == 0 {
					continue
				}

				By("checking " + node.Name)
				_, ok = node.Annotations[topolvm.CapacityKeyPrefix+classNameMap[node.Name]]
				Expect(ok).To(Equal(true), "capacity is not annotated: "+node.Name)
			}

			return nil
		}).Should(Succeed())
	})

}
