package framework

import (
	"fmt"
	"os"
	"strings"

	ginkgo "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (f *Framework) CreateInstancetype() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/instancetype.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreateClusterInstancetype() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/cluster-instancetype.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreatePreference() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/preference.yaml", "-n", f.Namespace.Name)
	return err
}

func (f *Framework) CreateClusterPreference() error {
	err := f.RunKubectlCommand("create", "-f", "manifests/cluster-preference.yaml", "-n", f.Namespace.Name)
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
	manifestFile := "manifests/blank_datavolume.yaml"
	if err := f.skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFile); err != nil {
		return err
	}
	return f.RunKubectlCreateYamlCommand(manifestFile)
}

func (f *Framework) CreateDataVolumeWithGuestAgentImage() error {
	manifestFile := "manifests/dv_with_guest_agent_image.yaml"
	if err := f.skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFile); err != nil {
		return err
	}
	return f.RunKubectlCreateYamlCommand(manifestFile)
}

func (f *Framework) CreatePVCUsingDataVolume() error {
	manifestFile := "manifests/dv_for_pvc.yaml"
	if err := f.skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFile); err != nil {
		return err
	}
	return f.RunKubectlCreateYamlCommand(manifestFile)
}

func (f *Framework) CreateVMWithInstancetypeAndPreference() error {
	manifestFile := "manifests/vm_with_instancetype_and_preference.yaml"
	if err := f.skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFile); err != nil {
		return err
	}
	return f.RunKubectlCreateYamlCommand(manifestFile)
}

func (f *Framework) CreateVMWithClusterInstancetypeAndClusterPreference() error {
	manifestFile := "manifests/vm_with_clusterinstancetype_and_clusterpreference.yaml"
	if err := f.skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFile); err != nil {
		return err
	}
	return f.RunKubectlCreateYamlCommand(manifestFile)
}

func (f *Framework) CreateVMWithDifferentVolumes() error {
	manifestFile := "manifests/vm_with_different_volume_types.yaml"
	if err := f.skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFile); err != nil {
		return err
	}
	return f.RunKubectlCreateYamlCommand(manifestFile)
}

func (f *Framework) CreateVMWithAccessCredentials() error {
	manifestFile := "manifests/vm_with_access_credentials.yaml"
	if err := f.skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFile); err != nil {
		return err

	}
	return f.RunKubectlCreateYamlCommand(manifestFile)
}

func (f *Framework) CreateVMWithDVAndDVTemplate() error {
	manifestFile := "manifests/vm_with_dv_and_dvtemplate.yaml"
	if err := f.skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFile); err != nil {
		return err
	}
	return f.RunKubectlCreateYamlCommand(manifestFile)
}

func (f *Framework) CreateVMWithPVC() error {
	manifestFile := "manifests/vm_with_pvc.yaml"
	if err := f.skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFile); err != nil {
		return err
	}
	return f.RunKubectlCommand("create", "-f", manifestFile, "-n", f.Namespace.Name)
}

func (f *Framework) CreateVMForHotplug() error {
	manifestFile := "manifests/vm_for_hotplug.yaml"
	if err := f.skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFile); err != nil {
		return err
	}
	return f.RunKubectlCreateYamlCommand(manifestFile)
}

func (f *Framework) CreateVMIWithDataVolume() error {
	manifestFile := "manifests/vmi_with_dv.yaml"
	if err := f.skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFile); err != nil {
		return err
	}
	return f.RunKubectlCommand("create", "-f", manifestFile, "-n", f.Namespace.Name)
}

// skipIfVolumeModeBlockIsRequestedAndNotSupported checks a manifest file for the "volumeMode: Block"
// string. If found, it verifies that the cluster supports block mode and skips the test if it does not.
func (f *Framework) skipIfVolumeModeBlockIsRequestedAndNotSupported(manifestFilePath string) error {
	content, err := os.ReadFile(manifestFilePath)
	if err != nil {
		return fmt.Errorf("failed to read manifest '%s': %w", manifestFilePath, err)
	}

	if strings.Contains(string(content), "volumeMode: Block") {
		// Assume IsVolumeModeBlockSupported is available from the other framework file
		isSupported, err := IsVolumeModeBlockSupported(f.KvClient, f.Namespace.Name, f.StorageClass)
		Expect(err).ToNot(HaveOccurred(), "Failed during prerequisite check for block volume support")
		// The Expect check will panic on failure, but we also handle the returned error for robustness.
		if err != nil {
			return fmt.Errorf("failed during prerequisite check for block volume support, sc: %s", f.StorageClass)
		}

		if !isSupported {
			ginkgo.Skip("Skipping test: manifest requires block volume mode, which is not supported by the current storage class.")
		}
	}

	return nil
}
