package tests_test

import (
	"kubevirt.io/kubevirt-velero-plugin/tests/reporters"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "KubeVirt Velero Plugin", reporters.NewReporters())
}
