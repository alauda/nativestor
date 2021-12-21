package e2e

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
)

func testCSIRawDevice() {
	It("should be deployed topolvm-scheduler pod", func() {

		ns := "test-raw-device"
		_, _, err := kubectl("create", "ns", ns)
		Expect(err).ShouldNot(HaveOccurred())

		pvc := `kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: pvc-raw-device
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 3Gi
  storageClassName: rawdevice-provisioner
  volumeMode: Block
`

		pod := `apiVersion: v1
kind: Pod
metadata:
  name: raw-test
  labels:
    app.kubernetes.io/name: raw-test
spec:
  containers:
    - name: ubuntu
      image: quay.io/cybozu/ubuntu:20.04
      command: ["/usr/local/bin/pause"]
      volumeDevices:
        - devicePath: /dev/sdf
          name: my-volume
  volumes:
    - name: my-volume
      persistentVolumeClaim:
        claimName: pvc-raw-device
`

		By("create pvc and pod to test raw-device")
		_, _, err = kubectlWithInput([]byte(pvc), "-n", ns, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, _, err = kubectlWithInput([]byte(pod), "-n", ns, "apply", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming that the pvc is bound")
		Eventually(func() error {
			stdout, stderr, err := kubectl("-n", ns, "get", "pvc", "pvc-raw-device", "-o=template", "--template={{.status.phase}}")
			if err != nil {
				return fmt.Errorf("failed to get pvc. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			phase := strings.TrimSpace(string(stdout))
			if phase != "Bound" {
				return fmt.Errorf("pvc %s is not bind", "pvc-raw-device")
			}
			return nil
		}).Should(Succeed())

		By("confirming that the pod is running")
		Eventually(func() error {
			stdout, stderr, err := kubectl("-n", ns, "get", "pod", "raw-test", "-o=template", "--template={{.status.phase}}")
			if err != nil {
				return fmt.Errorf("failed to get pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			phase := strings.TrimSpace(string(stdout))
			if phase != "Running" {
				return fmt.Errorf("pod %s not running may be mount failed", "raw-test")
			}
			return nil
		}).Should(Succeed())
	})
}
