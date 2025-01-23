package kvgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

func TestNewObjectRestoreGraph(t *testing.T) {
	// Helper function to create an Unstructured object from a given object
	toUnstructured := func(obj interface{}) (runtime.Unstructured, error) {
		unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, err
		}
		return &unstructured.Unstructured{Object: unstructuredObj}, nil
	}

	testCases := []struct {
		name           string
		object         interface{}
		expectedResult func(obj interface{}) ([]velero.ResourceIdentifier, error)
	}{
		{
			name: "VirtualMachine",
			object: &kvcore.VirtualMachine{
				TypeMeta: metav1.TypeMeta{
					Kind: "VirtualMachine",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-vm",
				},
				Spec: kvcore.VirtualMachineSpec{
					Preference: &kvcore.PreferenceMatcher{
						Name:         "test-preference",
						Kind:         "virtualmachinepreference",
						RevisionName: "controller-revision-preference",
					},
					Template: &kvcore.VirtualMachineInstanceTemplateSpec{
						Spec: kvcore.VirtualMachineInstanceSpec{
							Volumes: []kvcore.Volume{
								{
									Name: "test-volume",
									VolumeSource: kvcore.VolumeSource{
										DataVolume: &kvcore.DataVolumeSource{
											Name: "test-datavolume",
										},
									},
								},
							},
							AccessCredentials: []kvcore.AccessCredential{
								{
									SSHPublicKey: &kvcore.SSHPublicKeyAccessCredential{
										Source: kvcore.SSHPublicKeyAccessCredentialSource{
											Secret: &kvcore.AccessCredentialSecretSource{
												SecretName: "test-ssh-secret",
											},
										},
									},
								},
							},
						},
					},
				},
				Status: kvcore.VirtualMachineStatus{
					Created: true,
				},
			},
			expectedResult: func(obj interface{}) ([]velero.ResourceIdentifier, error) {
				return NewVirtualMachineRestoreGraph(obj.(*kvcore.VirtualMachine))
			},
		},
		{
			name: "VirtualMachineInstance",
			object: &kvcore.VirtualMachineInstance{
				TypeMeta: metav1.TypeMeta{
					Kind: "VirtualMachineInstance",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
				},
				Spec: kvcore.VirtualMachineInstanceSpec{
					Volumes: []kvcore.Volume{
						{
							VolumeSource: kvcore.VolumeSource{
								PersistentVolumeClaim: &kvcore.PersistentVolumeClaimVolumeSource{
									PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
										ClaimName: "test-pvc",
									},
								},
							},
						},
						{
							VolumeSource: kvcore.VolumeSource{
								ServiceAccount: &kvcore.ServiceAccountVolumeSource{
									ServiceAccountName: "test-sa",
								},
							},
						},
					},
					AccessCredentials: []kvcore.AccessCredential{
						{
							UserPassword: &kvcore.UserPasswordAccessCredential{
								Source: kvcore.UserPasswordAccessCredentialSource{
									Secret: &kvcore.AccessCredentialSecretSource{
										SecretName: "test-user-password",
									},
								},
							},
						},
					},
				},
			},
			expectedResult: func(obj interface{}) ([]velero.ResourceIdentifier, error) {
				return NewVirtualMachineInstanceRestoreGraph(obj.(*kvcore.VirtualMachineInstance))
			},
		},
		{
			name: "Pod",
			object: &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind: "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-pod",
				},
			},
			expectedResult: func(obj interface{}) ([]velero.ResourceIdentifier, error) {
				// Since there's no Pod-specific restore graph function, we return an empty list
				return []velero.ResourceIdentifier{}, nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			unstructuredObj, err := toUnstructured(tc.object)
			assert.NoError(t, err)

			expected, err := tc.expectedResult(tc.object)
			assert.NoError(t, err)
			actual, err := NewObjectRestoreGraph(unstructuredObj)
			assert.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestNewVirtualMachineRestoreGraph(t *testing.T) {
	getVM := func(created, backend bool) kvcore.VirtualMachine {
		return kvcore.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "",
				Name:      "test-vm",
			},
			Spec: kvcore.VirtualMachineSpec{
				Instancetype: &kvcore.InstancetypeMatcher{
					Name:         "test-instancetype",
					Kind:         "virtualmachineinstancetype",
					RevisionName: "controller-revision-instancetype",
				},
				Preference: &kvcore.PreferenceMatcher{
					Name:         "test-preference",
					Kind:         "virtualmachinepreference",
					RevisionName: "controller-revision-preference",
				},
				Template: &kvcore.VirtualMachineInstanceTemplateSpec{
					Spec: kvcore.VirtualMachineInstanceSpec{
						Volumes: []kvcore.Volume{
							{
								Name: "test-volume",
								VolumeSource: kvcore.VolumeSource{
									DataVolume: &kvcore.DataVolumeSource{
										Name: "test-datavolume",
									},
								},
							},
						},
						Domain: kvcore.DomainSpec{
							Devices: kvcore.Devices{
								TPM: &kvcore.TPMDevice{
									Persistent: &backend,
								},
							},
						},
						AccessCredentials: []kvcore.AccessCredential{
							{
								SSHPublicKey: &kvcore.SSHPublicKeyAccessCredential{
									Source: kvcore.SSHPublicKeyAccessCredentialSource{
										Secret: &kvcore.AccessCredentialSecretSource{
											SecretName: "test-ssh-secret",
										},
									},
								},
							},
						},
					},
				},
			},
			Status: kvcore.VirtualMachineStatus{
				Created: created,
			},
		}
	}
	testCases := []struct {
		name     string
		vm       kvcore.VirtualMachine
		expected []velero.ResourceIdentifier
	}{
		{"Should include all related resources",
			getVM(true, true),
			[]velero.ResourceIdentifier{
				{
					GroupResource: schema.GroupResource{Group: "instancetype.kubevirt.io", Resource: "virtualmachineinstancetype"},
					Namespace:     "",
					Name:          "test-instancetype",
				},
				{
					GroupResource: schema.GroupResource{Group: "apps", Resource: "controllerrevisions"},
					Namespace:     "",
					Name:          "controller-revision-instancetype",
				},
				{
					GroupResource: schema.GroupResource{Group: "instancetype.kubevirt.io", Resource: "virtualmachinepreference"},
					Namespace:     "",
					Name:          "test-preference",
				},
				{
					GroupResource: schema.GroupResource{Group: "apps", Resource: "controllerrevisions"},
					Namespace:     "",
					Name:          "controller-revision-preference",
				},
				{
					GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
					Namespace:     "",
					Name:          "test-datavolume",
				},
				{
					GroupResource: schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"},
					Namespace:     "",
					Name:          "backend-pvc",
				},
				{
					GroupResource: schema.GroupResource{Group: "", Resource: "secrets"},
					Namespace:     "",
					Name:          "test-ssh-secret",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			util.ListPVCs = func(labelSelector, ns string) (*v1.PersistentVolumeClaimList, error) {
				return &v1.PersistentVolumeClaimList{Items: []v1.PersistentVolumeClaim{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: ns,
							Name:      "backend-pvc",
							Labels: map[string]string{
								"persistent-state-for": "test-vm",
							},
						},
					},
				}}, nil
			}
			resources, err := NewVirtualMachineRestoreGraph(&tc.vm)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, resources)
		})
	}
}

func TestNewVirtualMachineInstanceRestoreGraph(t *testing.T) {
	testCases := []struct {
		name     string
		vmi      kvcore.VirtualMachineInstance
		expected []velero.ResourceIdentifier
	}{
		{"Should include all volumes",
			kvcore.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
				},
				Spec: kvcore.VirtualMachineInstanceSpec{
					Volumes: []kvcore.Volume{
						{
							VolumeSource: kvcore.VolumeSource{
								PersistentVolumeClaim: &kvcore.PersistentVolumeClaimVolumeSource{
									PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
										ClaimName: "test-pvc",
									},
								},
							},
						},
						{
							VolumeSource: kvcore.VolumeSource{
								DataVolume: &kvcore.DataVolumeSource{
									Name: "test-dv",
								},
							},
						},
						{
							VolumeSource: kvcore.VolumeSource{
								MemoryDump: &kvcore.MemoryDumpVolumeSource{
									PersistentVolumeClaimVolumeSource: kvcore.PersistentVolumeClaimVolumeSource{
										PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
											ClaimName: "test-memoryDump",
										},
									},
								},
							},
						},
						{
							VolumeSource: kvcore.VolumeSource{
								ConfigMap: &kvcore.ConfigMapVolumeSource{
									LocalObjectReference: v1.LocalObjectReference{
										Name: "test-cm",
									},
								},
							},
						},
						{
							VolumeSource: kvcore.VolumeSource{
								Secret: &kvcore.SecretVolumeSource{
									SecretName: "test-secret",
								},
							},
						},
						{
							VolumeSource: kvcore.VolumeSource{
								ServiceAccount: &kvcore.ServiceAccountVolumeSource{
									ServiceAccountName: "test-sa",
								},
							},
						},
					},
				},
			},
			[]velero.ResourceIdentifier{
				{
					GroupResource: kuberesource.PersistentVolumeClaims,
					Namespace:     "test-namespace",
					Name:          "test-pvc",
				},
				{
					GroupResource: schema.GroupResource{
						Group:    "cdi.kubevirt.io",
						Resource: "datavolumes",
					},
					Namespace: "test-namespace",
					Name:      "test-dv",
				},
				{
					GroupResource: kuberesource.PersistentVolumeClaims,
					Namespace:     "test-namespace",
					Name:          "test-memoryDump",
				},
				{
					GroupResource: schema.GroupResource{
						Group:    "",
						Resource: "configmaps",
					},
					Namespace: "test-namespace",
					Name:      "test-cm",
				},
				{
					GroupResource: kuberesource.Secrets,
					Namespace:     "test-namespace",
					Name:          "test-secret",
				},
				{
					GroupResource: kuberesource.ServiceAccounts,
					Namespace:     "test-namespace",
					Name:          "test-sa",
				},
			},
		},
		{"Should include all access credentials",
			kvcore.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
				},
				Spec: kvcore.VirtualMachineInstanceSpec{
					AccessCredentials: []kvcore.AccessCredential{
						{
							SSHPublicKey: &kvcore.SSHPublicKeyAccessCredential{
								Source: kvcore.SSHPublicKeyAccessCredentialSource{
									Secret: &kvcore.AccessCredentialSecretSource{
										SecretName: "test-ssh-public-key",
									},
								},
							},
						},
						{
							UserPassword: &kvcore.UserPasswordAccessCredential{
								Source: kvcore.UserPasswordAccessCredentialSource{
									Secret: &kvcore.AccessCredentialSecretSource{
										SecretName: "test-user-password",
									},
								},
							},
						},
					},
				},
			},
			[]velero.ResourceIdentifier{
				{
					GroupResource: kuberesource.Secrets,
					Namespace:     "test-namespace",
					Name:          "test-ssh-public-key",
				},
				{
					GroupResource: kuberesource.Secrets,
					Namespace:     "test-namespace",
					Name:          "test-user-password",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := NewVirtualMachineInstanceRestoreGraph(&tc.vmi)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, output)
		})
	}
}
