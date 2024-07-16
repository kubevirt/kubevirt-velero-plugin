package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func TestPodRestoreExecute(t *testing.T) {
	testCases := []struct {
		name  string
		input velero.RestoreItemActionExecuteInput
	}{
		{"Always skip the pod",
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

func TestPodRestoreApplyTo(t *testing.T) {
	testCases := []struct {
		name        string
		shouldMatch bool
		pod         core.Pod
	}{
		{"Match launcher pod",
			true,
			core.Pod{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{"kubevirt.io": "virt-launcher"},
				},
			},
		},
		{"Match hotplug pod",
			true,
			core.Pod{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{"kubevirt.io": "hotplug-disk"},
				},
			},
		},
		{"Don't match non-launcher pods",
			false,
			core.Pod{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{},
				},
			},
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	action := NewPodRestoreItemAction(logrus.StandardLogger())
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selector, _ := action.AppliesTo()
			parsed, err := labels.Parse(selector.LabelSelector)
			assert.Nil(t, err)

			if tc.shouldMatch {
				assert.True(t, parsed.Matches(labels.Set(tc.pod.Labels)))
			} else {
				assert.False(t, parsed.Matches(labels.Set(tc.pod.Labels)))
			}
		})
	}
}
