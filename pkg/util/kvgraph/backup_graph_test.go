package kvgraph

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kvcore "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/kubevirt-velero-plugin/pkg/util"
)

func TestNewObjectBackupGraph(t *testing.T) {
	// Create an Unstructured object from a given object
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
						},
					},
				},
				Status: kvcore.VirtualMachineStatus{
					Created: true,
				},
			},
			expectedResult: func(obj interface{}) ([]velero.ResourceIdentifier, error) {
				return NewVirtualMachineBackupGraph(obj.(*kvcore.VirtualMachine))
			},
		},
		{
			name: "VirtualMachineInstance",
			object: &kvcore.VirtualMachineInstance{
				TypeMeta: metav1.TypeMeta{
					Kind: "VirtualMachineInstance",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vmi",
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
					},
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
					},
				},
			},
			expectedResult: func(obj interface{}) ([]velero.ResourceIdentifier, error) {
				return NewVirtualMachineInstanceBackupGraph(obj.(*kvcore.VirtualMachineInstance))
			},
		},
		{
			name: "DataVolume",
			object: &cdiv1.DataVolume{
				TypeMeta: metav1.TypeMeta{
					Kind: "DataVolume",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dv",
					Namespace: "default",
				},
				Status: cdiv1.DataVolumeStatus{
					Phase: cdiv1.Succeeded,
				},
			},
			expectedResult: func(obj interface{}) ([]velero.ResourceIdentifier, error) {
				return NewDataVolumeBackupGraph(obj.(*cdiv1.DataVolume)), nil
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
				// Since there's no Pod-specific backup graph function, we return an empty list
				return []velero.ResourceIdentifier{}, nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			util.ListPods = func(name, ns string) (*v1.PodList, error) {
				return &v1.PodList{Items: []v1.Pod{}}, nil
			}

			unstructuredObj, err := toUnstructured(tc.object)
			assert.NoError(t, err)

			expected, err := tc.expectedResult(tc.object)
			assert.NoError(t, err)

			actual, err := NewObjectBackupGraph(unstructuredObj)
			assert.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestNewVirtualMachineBackupGraph(t *testing.T) {
	getVM := func(created, backend bool) kvcore.VirtualMachine {
		return kvcore.VirtualMachine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "",
				Name:      "test-vm",
			},
			Spec: kvcore.VirtualMachineSpec{
				Instancetype: &kvcore.InstancetypeMatcher{
					Name: "test-instancetype",
					Kind: "virtualmachineinstancetype",
				},
				Preference: &kvcore.PreferenceMatcher{
					Name: "test-preference",
					Kind: "virtualmachinepreference",
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
				InstancetypeRef: &kvcore.InstancetypeStatusRef{
					Name: "test-instancetype",
					Kind: "virtualmachineinstancetype",
					ControllerRevisionRef: &kvcore.ControllerRevisionRef{
						Name: "controller-revision-instancetype",
					},
				},
				PreferenceRef: &kvcore.InstancetypeStatusRef{
					Name: "test-preference",
					Kind: "virtualmachinepreference",
					ControllerRevisionRef: &kvcore.ControllerRevisionRef{
						Name: "controller-revision-preference",
					},
				},
			},
		}
	}
	testCases := []struct {
		name     string
		vm       kvcore.VirtualMachine
		expected []velero.ResourceIdentifier
	}{
		{"Should include all related resources",
			getVM(true, false),
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
					GroupResource: schema.GroupResource{Group: "kubevirt.io", Resource: "virtualmachineinstances"},
					Namespace:     "",
					Name:          "test-vm",
				},
				{
					GroupResource: schema.GroupResource{Group: "", Resource: "pods"},
					Namespace:     "",
					Name:          "test-vmi-launcher-pod",
				},
				{
					GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
					Namespace:     "",
					Name:          "test-datavolume",
				},
				{
					GroupResource: schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"},
					Namespace:     "",
					Name:          "test-datavolume",
				},
				{
					GroupResource: schema.GroupResource{Group: "", Resource: "secrets"},
					Namespace:     "",
					Name:          "test-ssh-secret",
				},
			},
		},
		{"Should not include vmi and launcher pod",
			getVM(false, false),
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
					Name:          "test-datavolume",
				},
				{
					GroupResource: schema.GroupResource{Group: "", Resource: "secrets"},
					Namespace:     "",
					Name:          "test-ssh-secret",
				},
			},
		},
		{"Should include backend PVC",
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
					GroupResource: schema.GroupResource{Group: "kubevirt.io", Resource: "virtualmachineinstances"},
					Namespace:     "",
					Name:          "test-vm",
				},
				{
					GroupResource: schema.GroupResource{Group: "", Resource: "pods"},
					Namespace:     "",
					Name:          "test-vmi-launcher-pod",
				},
				{
					GroupResource: schema.GroupResource{Group: "cdi.kubevirt.io", Resource: "datavolumes"},
					Namespace:     "",
					Name:          "test-datavolume",
				},
				{
					GroupResource: schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"},
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
			util.ListPods = func(name, ns string) (*v1.PodList, error) {
				return &v1.PodList{Items: []v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "test-namespace",
							Name:      "test-vmi-launcher-pod",
							Labels: map[string]string{
								"kubevirt.io": "virt-launcher",
							},
							Annotations: map[string]string{
								"kubevirt.io/domain": "test-vm",
							},
						},
					},
				}}, nil
			}
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
			resources, err := NewVirtualMachineBackupGraph(&tc.vm)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, resources)
		})
	}
}

func TestNewVirtualMachineInstanceBackupGraph(t *testing.T) {
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
					Name:          "test-dv",
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
			output, err := NewVirtualMachineInstanceBackupGraph(&tc.vmi)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, output)
		})
	}
}

func TestAddLauncherPod(t *testing.T) {
	testCases := []struct {
		name     string
		vmi      kvcore.VirtualMachineInstance
		pods     []v1.Pod
		expected []velero.ResourceIdentifier
	}{
		{"Should include launcher pod if present",
			kvcore.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-vmi",
				},
			},
			[]v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-vmi-launcher-pod",
						Labels: map[string]string{
							"kubevirt.io": "virt-launcher",
						},
						Annotations: map[string]string{
							"kubevirt.io/domain": "test-vmi",
						},
					},
				},
			},
			[]velero.ResourceIdentifier{
				{
					GroupResource: kuberesource.Pods,
					Namespace:     "test-namespace",
					Name:          "test-vmi-launcher-pod",
				},
			},
		},
		{"Should include only own launcher pod",
			kvcore.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-vmi",
				},
			},
			[]v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-vmi-launcher-pod",
						Labels: map[string]string{
							"kubevirt.io": "virt-launcher",
						},
						Annotations: map[string]string{
							"kubevirt.io/domain": "another-vmi",
						},
					},
				},
			},
			[]velero.ResourceIdentifier{},
		},
		{"Should not include other pods",
			kvcore.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-vmi",
				},
			},
			[]v1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test-namespace",
						Name:      "test-vmi-launcher-pod",
					},
				},
			},
			[]velero.ResourceIdentifier{},
		},
	}

	logrus.SetLevel(logrus.ErrorLevel)
	kubecli.GetKubevirtClientFromClientConfig = kubecli.GetMockKubevirtClientFromClientConfig
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			util.ListPods = func(name, ns string) (*v1.PodList, error) {
				return &v1.PodList{Items: tc.pods}, nil
			}
			vmi := &tc.vmi
			output, err := addLauncherPod(vmi.GetName(), vmi.GetNamespace(), []velero.ResourceIdentifier{})
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, output)
		})
	}
}

func TestNewDataVolumeBackupGraph(t *testing.T) {
	tests := []struct {
		name           string
		dataVolume     *cdiv1.DataVolume
		expectedResult []velero.ResourceIdentifier
	}{
		{
			name: "DataVolume Succeeded",
			dataVolume: &cdiv1.DataVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dv",
					Namespace: "default",
				},
				Status: cdiv1.DataVolumeStatus{
					Phase: cdiv1.Succeeded,
				},
			},
			expectedResult: []velero.ResourceIdentifier{
				{
					GroupResource: schema.GroupResource{
						Group:    "",
						Resource: "persistentvolumeclaims",
					},
					Namespace: "default",
					Name:      "test-dv",
				},
			},
		},
		{
			name: "DataVolume Not Succeeded",
			dataVolume: &cdiv1.DataVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-dv",
					Namespace: "default",
				},
				Status: cdiv1.DataVolumeStatus{
					Phase: cdiv1.ImportScheduled,
				},
			},
			expectedResult: []velero.ResourceIdentifier{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewDataVolumeBackupGraph(tt.dataVolume)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
