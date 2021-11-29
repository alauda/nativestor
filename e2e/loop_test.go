package e2e

import (
	"encoding/json"
	"fmt"

	topolvmv2 "github.com/alauda/topolvm-operator/api/v2"
	"github.com/alauda/topolvm-operator/pkg/cluster"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func testLoop() {
	It("topolvm cluster should be deleted", func() {
		Eventually(func() error {
			kubectl("-n=topolvm-system", "delete", "topolvmcluster", "topolvmcluster-sample")
			result, stderr, err := kubectl("-n=topolvm-system", "get", "deployment", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}
			var deploymentList appsv1.DeploymentList
			err = json.Unmarshal(result, &deploymentList)
			if err != nil {
				return err
			}
			if len(deploymentList.Items) != 1 {
				return fmt.Errorf("the number of topolvm-node deployment is not equal to 1: %d", len(deploymentList.Items))
			}
			return nil
		}).Should(Succeed())
	})

	It("topolvm cluster should be created", func() {

		topolvmCluster := `apiVersion: topolvm.cybozu.com/v2
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  namespace: topolvm-system
spec:
  topolvmVersion: "alaudapublic/topolvm:2.0.0"
  storage:
    useAllNodes: false
    useAllDevices: false
    useLoop: true
    deviceClasses:
      - nodeName: "topolvm-e2e-worker"
        classes:
          - className: "hdd1"
            volumeGroup: "node1-myvg1-auto"
            default: true
            devices:
              - name: "loop0"
                type: "loop"
                auto: true
                path: "/var/lib/"
                size: 2
      - nodeName: "topolvm-e2e-worker2"
        classes:
          - className: "hdd2"
            volumeGroup: "node2-myvg1-auto"
            default: true
            devices:
              - name: "loop0"
                type: "loop"
                auto: true
                path: "/var/lib/"
                size: 2
      - nodeName: "topolvm-e2e-worker3"
        classes:
          - className: "hdd3"
            volumeGroup: "node3-myvg1-auto"
            default: true
            devices:
              - name: "loop0"
                type: "loop"
                auto: true
                path: "/var/lib/"
                size: 2
`

		Eventually(func() error {
			_, _, err := kubectlWithInput([]byte(topolvmCluster), "create", "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())
			return nil
		}).Should(Succeed())
	})

	It("lvmd configmap should be created", func() {
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
			if len(cmList.Items) != 4 {
				for _, ele := range cmList.Items {
					fmt.Printf("%s: %s \n", cluster.LvmdConfigMapKey, ele.Data[cluster.LvmdConfigMapKey])
					fmt.Printf("%s: %s \n", cluster.VgStatusConfigMapKey, ele.Data[cluster.VgStatusConfigMapKey])
					fmt.Printf("%s: %s \n", cluster.LocalDiskCMData, ele.Data[cluster.LocalDiskCMData])
					result, stderr, err := execAtLocal("losetup", nil, "-a")
					if err != nil {
						return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
					}
					fmt.Printf("loop devices %s", string(result))
				}
				return fmt.Errorf("the number of topolvm-node deployment is not equal to 4: %d", len(cmList.Items))
			}

			By("checking lvmd classname")
			for _, cm := range cmList.Items {
				if cm.GetAnnotations()[cluster.LvmdAnnotationsNodeKey] == "topolvm-e2e-control-plane" {
					continue
				}
				nodeStatus := &topolvmv2.NodeStorageState{}
				err = json.Unmarshal([]byte(cm.Data[cluster.VgStatusConfigMapKey]), nodeStatus)
				if err != nil {
					return err
				}
				for _, ele := range nodeStatus.Loops {
					if ele.Status != cluster.LoopCreateSuccessful {
						return fmt.Errorf("loop %s create failed", ele.Name)
					}
				}
			}
			return nil
		}).Should(Succeed())
	})

	It("node deployment should be created", func() {
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
}
