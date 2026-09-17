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

package kvgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

func TestIsParameterized(t *testing.T) {
	assert.True(t, IsParameterized("${NAME}"))
	assert.True(t, IsParameterized("rootdisk-${NAME}"))
	assert.False(t, IsParameterized("rootdisk"))
	assert.False(t, IsParameterized(""))
}

func TestAddVeleroResourceSkipsParameterizedValues(t *testing.T) {
	testCases := []struct {
		name      string
		itemName  string
		namespace string
		expected  []velero.ResourceIdentifier
	}{
		{"concrete name and namespace",
			"my-dv", "my-ns",
			[]velero.ResourceIdentifier{{GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"}, Namespace: "my-ns", Name: "my-dv"}},
		},
		{"parameterized name", "dv-${NAME}", "my-ns", nil},
		{"parameterized namespace", "my-dv", "${NAMESPACE}", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := addVeleroResource(tc.itemName, tc.namespace, "datavolumes", nil)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestAddDataVolumeTemplateGraph(t *testing.T) {
	origGetDV := util.GetDV
	defer func() { util.GetDV = origGetDV }()

	testCases := []struct {
		name      string
		dvts      []v1.DataVolumeTemplateSpec
		namespace string
		dvExists  bool
		expected  []velero.ResourceIdentifier
	}{
		{"PVC source, no explicit namespace, backing DataVolume exists",
			[]v1.DataVolumeTemplateSpec{
				{Spec: cdiv1.DataVolumeSpec{Source: &cdiv1.DataVolumeSource{PVC: &cdiv1.DataVolumeSourcePVC{Name: "golden-image"}}}},
			},
			"tpl-ns",
			true,
			[]velero.ResourceIdentifier{
				{GroupResource: schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"}, Namespace: "tpl-ns", Name: "golden-image"},
				{GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"}, Namespace: "tpl-ns", Name: "golden-image"},
			},
		},
		{"PVC source, explicit namespace, backing DataVolume exists",
			[]v1.DataVolumeTemplateSpec{
				{Spec: cdiv1.DataVolumeSpec{Source: &cdiv1.DataVolumeSource{PVC: &cdiv1.DataVolumeSourcePVC{Namespace: "other-ns", Name: "golden-image"}}}},
			},
			"tpl-ns",
			true,
			[]velero.ResourceIdentifier{
				{GroupResource: schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"}, Namespace: "other-ns", Name: "golden-image"},
				{GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"}, Namespace: "other-ns", Name: "golden-image"},
			},
		},
		{"PVC source with no matching DataVolume - only the PVC is added",
			[]v1.DataVolumeTemplateSpec{
				{Spec: cdiv1.DataVolumeSpec{Source: &cdiv1.DataVolumeSource{PVC: &cdiv1.DataVolumeSourcePVC{Name: "hand-authored-pvc"}}}},
			},
			"tpl-ns",
			false,
			[]velero.ResourceIdentifier{
				{GroupResource: schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"}, Namespace: "tpl-ns", Name: "hand-authored-pvc"},
			},
		},
		{"Snapshot source",
			[]v1.DataVolumeTemplateSpec{
				{Spec: cdiv1.DataVolumeSpec{Source: &cdiv1.DataVolumeSource{Snapshot: &cdiv1.DataVolumeSourceSnapshot{Name: "golden-snap"}}}},
			},
			"tpl-ns",
			false,
			[]velero.ResourceIdentifier{
				{GroupResource: schema.GroupResource{Group: "snapshot.storage.k8s.io", Resource: "volumesnapshots"}, Namespace: "tpl-ns", Name: "golden-snap"},
			},
		},
		{"DataSource sourceRef",
			[]v1.DataVolumeTemplateSpec{
				{Spec: cdiv1.DataVolumeSpec{SourceRef: &cdiv1.DataVolumeSourceRef{Kind: "DataSource", Name: "golden-ds"}}},
			},
			"tpl-ns",
			false,
			[]velero.ResourceIdentifier{
				{GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datasources"}, Namespace: "tpl-ns", Name: "golden-ds"},
			},
		},
		{"Parameterized PVC name is skipped",
			[]v1.DataVolumeTemplateSpec{
				{Spec: cdiv1.DataVolumeSpec{Source: &cdiv1.DataVolumeSource{PVC: &cdiv1.DataVolumeSourcePVC{Name: "rootdisk-${NAME}"}}}},
			},
			"tpl-ns",
			true,
			nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			util.GetDV = func(ns, name string) (*cdiv1.DataVolume, error) {
				if tc.dvExists {
					return &cdiv1.DataVolume{}, nil
				}
				return nil, apierrors.NewNotFound(schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"}, name)
			}
			result, err := addDataVolumeTemplateGraph(tc.dvts, tc.namespace, true, nil)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestAddDataVolumeTemplateGraphDataVolumeLookupFails(t *testing.T) {
	origGetDV := util.GetDV
	defer func() { util.GetDV = origGetDV }()
	// An error that is not NotFound is no evidence the DataVolume is absent, so it must not
	// be mistaken for a PVC-only source - that would silently leave the golden image's
	// DataVolume out of the backup.
	util.GetDV = func(ns, name string) (*cdiv1.DataVolume, error) {
		return nil, apierrors.NewForbidden(schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"}, name, assert.AnError)
	}

	dvts := []v1.DataVolumeTemplateSpec{
		{Spec: cdiv1.DataVolumeSpec{Source: &cdiv1.DataVolumeSource{PVC: &cdiv1.DataVolumeSourcePVC{Name: "golden-image"}}}},
	}

	_, err := addDataVolumeTemplateGraph(dvts, "tpl-ns", true, nil)
	assert.Error(t, err)
}

func TestAddDataVolumeTemplateGraphSkipsLookupOnRestore(t *testing.T) {
	origGetDV := util.GetDV
	defer func() { util.GetDV = origGetDV }()
	// On restore the DataVolume has not been created in the target cluster yet, so looking
	// it up would always report "absent" and drop it from the restore.
	util.GetDV = func(ns, name string) (*cdiv1.DataVolume, error) {
		t.Fatalf("GetDV must not be called on the restore path")
		return nil, nil
	}

	dvts := []v1.DataVolumeTemplateSpec{
		{Spec: cdiv1.DataVolumeSpec{Source: &cdiv1.DataVolumeSource{PVC: &cdiv1.DataVolumeSourcePVC{Name: "golden-image"}}}},
	}

	result, err := addDataVolumeTemplateGraph(dvts, "tpl-ns", false, nil)
	assert.NoError(t, err)
	assert.Equal(t, []velero.ResourceIdentifier{
		{GroupResource: schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"}, Namespace: "tpl-ns", Name: "golden-image"},
		{GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"}, Namespace: "tpl-ns", Name: "golden-image"},
	}, result)
}

func TestAddTemplateInstancetypeGraph(t *testing.T) {
	testCases := []struct {
		name     string
		vm       v1.VirtualMachine
		expected []velero.ResourceIdentifier
	}{
		{"Namespaced instancetype and preference are included",
			v1.VirtualMachine{
				Spec: v1.VirtualMachineSpec{
					Instancetype: &v1.InstancetypeMatcher{Name: "my-instancetype", Kind: "virtualmachineinstancetype"},
					Preference:   &v1.PreferenceMatcher{Name: "my-preference", Kind: "virtualmachinepreference"},
				},
			},
			[]velero.ResourceIdentifier{
				{GroupResource: schema.GroupResource{Group: "instancetype.kubevirt.io", Resource: "virtualmachineinstancetypes"}, Namespace: "tpl-ns", Name: "my-instancetype"},
				{GroupResource: schema.GroupResource{Group: "instancetype.kubevirt.io", Resource: "virtualmachinepreferences"}, Namespace: "tpl-ns", Name: "my-preference"},
			},
		},
		{"Cluster-scoped (default) matchers are not included",
			v1.VirtualMachine{
				Spec: v1.VirtualMachineSpec{
					Instancetype: &v1.InstancetypeMatcher{Name: "u1.medium"},
					Preference:   &v1.PreferenceMatcher{Name: "fedora"},
				},
			},
			nil,
		},
		{"Cluster-scoped matchers with explicit Kind are not included",
			v1.VirtualMachine{
				Spec: v1.VirtualMachineSpec{
					Instancetype: &v1.InstancetypeMatcher{Name: "u1.medium", Kind: "virtualmachineclusterinstancetype"},
				},
			},
			nil,
		},
		{"No matchers", v1.VirtualMachine{}, nil},
		{"The template's namespace wins over the embedded VirtualMachine's",
			v1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{Namespace: "hardcoded-ns"},
				Spec: v1.VirtualMachineSpec{
					Instancetype: &v1.InstancetypeMatcher{Name: "my-instancetype", Kind: "virtualmachineinstancetype"},
				},
			},
			[]velero.ResourceIdentifier{
				{GroupResource: schema.GroupResource{Group: "instancetype.kubevirt.io", Resource: "virtualmachineinstancetypes"}, Namespace: "tpl-ns", Name: "my-instancetype"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := addTemplateInstancetypeGraph(&tc.vm, "tpl-ns", nil)
			assert.Equal(t, tc.expected, result)
		})
	}
}
