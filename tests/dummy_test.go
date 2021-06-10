package tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dummy test", func() {
	Context("Testing the testing", func() {
		It("Should build and run the test", func() {
			Expect(true)
		})
	})
})
