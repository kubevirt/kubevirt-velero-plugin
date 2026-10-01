/*
 * This file is part of the Kubevirt Velero Plugin project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Velero Plugin Authors.
 *
 */

package plugin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestVMTRRestoreItemAction_AppliesTo(t *testing.T) {
	action := NewVMTRRestoreItemAction(logrus.StandardLogger())

	selector, err := action.AppliesTo()
	assert.NoError(t, err)
	assert.Contains(t, selector.IncludedResources, "virtualmachinetemplaterequests.template.kubevirt.io")
}

func TestVMTRRestoreItemAction_Execute(t *testing.T) {
	logrus.SetLevel(logrus.ErrorLevel)
	action := NewVMTRRestoreItemAction(logrus.StandardLogger())

	t.Run("nil input returns error", func(t *testing.T) {
		_, err := action.Execute(nil)
		assert.Error(t, err)
	})

	t.Run("always skips restore", func(t *testing.T) {
		input := &velero.RestoreItemActionExecuteInput{
			Item: &unstructured.Unstructured{Object: map[string]interface{}{}},
		}

		output, err := action.Execute(input)
		assert.NoError(t, err)
		assert.True(t, output.SkipRestore)
	})
}
