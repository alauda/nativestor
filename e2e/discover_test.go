package e2e

import (
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
)

func testDiscover() {
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

		topolvmCluster := `apiVersion: topolvm.cybozu.com/v1
kind: TopolvmCluster
metadata:
  name: topolvmcluster-sample
  namespace: topolvm-system
spec:
  topolvmVersion: "alaudapublic/topolvm:1.0.0"
  storage:
    useAllNodes: true
    useAllDevices: true
    useLoop: true
    volumeGroupName: discover
    className: hdd
`

		Eventually(func() error {
			_, _, err := kubectlWithInput([]byte(topolvmCluster), "create", "-f", "-")
			Expect(err).ShouldNot(HaveOccurred())
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
			if len(deploymentList.Items) < 1 {
				return fmt.Errorf("the number of topolvm-node deployment is not equal to 3: %d", len(deploymentList.Items))
			}
			return nil
		}).Should(Succeed())
	})
}
