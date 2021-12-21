package e2e

import (
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func testCSIController() {
	It("should be deployed", func() {
		Eventually(func() error {
			result, stderr, err := kubectl("get", "-n=topolvm-system", "pod", "--selector=app.kubernetes.io/name=topolvm-controller", "-o=json")
			if err != nil {
				return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
			}
			var pods corev1.PodList
			err = json.Unmarshal(result, &pods)
			if err != nil {
				return err
			}
			if len(pods.Items) != 2 {
				fmt.Printf(" topolvm controller pod should be 2 but %d \n", len(pods.Items))
				return fmt.Errorf(" topolvm controller pod should be 2 but %d \n", len(pods.Items))
			}
			for _, ele := range pods.Items {
				err = checkPodReady(&ele)
				if err != nil {
					fmt.Printf("the node pod status %s", pods.Items[0].Status.Phase)
					return fmt.Errorf("the node pod status %s", pods.Items[0].Status.Phase)
				}
			}

			return nil

		}).Should(Succeed())
	})
}
