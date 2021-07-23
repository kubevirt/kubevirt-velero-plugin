package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestVmiRestoreExecute(t *testing.T) {
	testCases := []struct {
		name  string
		input velero.RestoreItemActionExecuteInput
	}{
		{"Always skip the VMI",
			velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: map[string]interface{}{},
				},
			},
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewPodRestoreItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, _ := action.Execute(&tc.input)

			assert.True(t, output.SkipRestore)
		})
	}
}
