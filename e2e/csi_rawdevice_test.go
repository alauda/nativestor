package e2e

import (
	"encoding/json"
	"fmt"
	rawv1 "github.com/alauda/nativestor/apis/rawdevice/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"strings"
)

func testCSIRawDevice() {
	It("start test raw device feature", func() {
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

		stdout, _, err := kubectl("-n", ns, "get", "pvc", "pvc-raw-device", "-o", "json")
		Expect(err).ShouldNot(HaveOccurred())
		var p v1.PersistentVolumeClaim
		err = json.Unmarshal(stdout, &p)
		Expect(err).ShouldNot(HaveOccurred())

		stdout, _, err = kubectl("get", "pv", p.Spec.VolumeName, "-o", "json")
		Expect(err).ShouldNot(HaveOccurred())
		var pv v1.PersistentVolume
		err = json.Unmarshal(stdout, &pv)
		Expect(err).ShouldNot(HaveOccurred())

		By("delete pod and pvc")
		_, _, err = kubectlWithInput([]byte(pod), "-n", ns, "delete", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())
		_, _, err = kubectlWithInput([]byte(pvc), "-n", ns, "delete", "-f", "-")
		Expect(err).ShouldNot(HaveOccurred())

		By("confirming the raw device has gc")

		Eventually(func() error {
			stdout, stderr, err := kubectl("get", "rawdevice", pv.Spec.CSI.VolumeHandle, "-o", "json")
			if err != nil {
				return fmt.Errorf("failed to get pod. stdout: %s, stderr: %s, err: %v", stdout, stderr, err)
			}
			var rawdevice rawv1.RawDevice
			err = json.Unmarshal(stdout, &rawdevice)
			if err != nil {
				return err
			}
			if rawdevice.Status.Name != "" {
				return errors.New("raw device status.name should empty")
			}
			return nil
		}).Should(Succeed())
	})
}

func checkRawDevicAvailableCountBefore() uint32 {

	result, _, err := kubectl("get", "rawdevice", "-o=json")
	Expect(err).ShouldNot(HaveOccurred())
	var rawdevices rawv1.RawDeviceList
	err = json.Unmarshal(result, &rawdevices)
	Expect(err).ShouldNot(HaveOccurred())
	var c uint32
	for _, raw := range rawdevices.Items {
		if raw.Spec.Available && raw.Status.Name == "" {
			c++
		}
	}
	return c
}

func checkRawDevicAvailableCountAfter(count uint32) {
	Eventually(func() error {
		result, stderr, err := kubectl("get", "rawdevice", "-o=json")
		if err != nil {
			return fmt.Errorf("%v: stdout=%s, stderr=%s", err, result, stderr)
		}
		var rawdevices rawv1.RawDeviceList
		err = json.Unmarshal(result, &rawdevices)
		if err != nil {
			return err
		}
		var c uint32
		for _, raw := range rawdevices.Items {
			if raw.Spec.Available && raw.Status.Name == "" {
				c++
			}
		}
		if count != c {
			return fmt.Errorf("raw devie available before is %d after is %d", count, c)
		}
		return nil
	}).Should(Succeed())
}
