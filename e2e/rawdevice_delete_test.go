package e2e

import (
	"encoding/json"
	"github.com/pkg/errors"
	"strings"

	rawv1 "github.com/alauda/nativestor/apis/rawdevice/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func testRawDeviceDelete() {
	It("test raw device delete", func() {
		stdout, _, err := kubectl("get", "rawdevice", "-o", "json")
		Expect(err).ShouldNot(HaveOccurred())
		var rawdevices rawv1.RawDeviceList
		err = json.Unmarshal(stdout, &rawdevices)
		var rawdevice rawv1.RawDevice
		for _, dev := range rawdevices.Items {
			if dev.Spec.Available && dev.Status.Name == "" {
				rawdevice = dev
			}
		}

		By("delete device " + rawdevice.Spec.RealPath)
		_, _, err = execAtLocal("losetup", nil, "-d", rawdevice.Spec.RealPath)
		Expect(err).ShouldNot(HaveOccurred())

		By("checking rawdevice deleted")
		Eventually(func() error {
			_, stderr, err := kubectl("get", "rawdevice", rawdevice.Name)
			if err != nil {
				if strings.Contains(string(stderr), "NotFound") {
					return nil
				}
				return err
			}
			return errors.New("raw device should be deleted")
		}).Should(Succeed())
	})
}
