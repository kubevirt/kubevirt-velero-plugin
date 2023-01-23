package framework

func (f *Framework) CreateInstancetype() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/instancetype.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreatePreference() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/preference.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreateConfigMap() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/configmap.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreateSecret() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/secret.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreateAccessCredentialsSecret() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/accessCredentialsSecret.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreateBlankDataVolume() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/blank_datavolume.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreatePVCUsingDataVolume() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/dv-for-pvc.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreateVMWithInstancetypeAndPreference() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/vm_with_instancetype_and_preference.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreateVMWithDifferentVolumes() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/vm_with_different_volume_types.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreateVMWithAccessCredentials() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/vm_with_access_credentials.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreateVMWithDVAndDVTemplate() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/vm_with_dv_and_dvtemplate.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreateVMWithPVC() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/vm_with_pvc.yaml", "-n", f.Namespace.Name)
	return err
}
