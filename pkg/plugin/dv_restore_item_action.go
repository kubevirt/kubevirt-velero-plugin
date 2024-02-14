package plugin

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// DVRestoreItemAction is a backup item action for restoring DataVolumes
type DVRestoreItemAction struct {
	log logrus.FieldLogger
}

// NewDVRestoreItemAction instantiates a DataVolume.
func NewDVRestoreItemAction(log logrus.FieldLogger) *DVRestoreItemAction {
	return &DVRestoreItemAction{log: log}
}

// AppliesTo returns information about which resources this action should be invoked for.
func (p *DVRestoreItemAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
			IncludedResources: []string{"DataVolume"},
		},
		nil
}

// Execute if the DV and the corresponding DV is not SUCCESSFULL - then skip PVC
func (p *DVRestoreItemAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input object nil!")
	}

	var dv cdiv1.DataVolume
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), &dv); err != nil {
		return nil, errors.WithStack(err)
	}
	p.log.Infof("handling DV %v/%v", dv.GetNamespace(), dv.GetName())
	annotations := dv.GetAnnotations()
	value, hasLabelSelector := annotations[AnnPrePopulated]

	if dv.Spec.Source.PVC != nil {
		resetDVSpec(&dv)
	}

	if hasLabelSelector && value == dv.Name {
		delete(dv.Annotations, AnnPrePopulated)
	}

	dvMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&dv)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	p.log.Info("Returning from DVRestoreItemAction for DV")
	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     &unstructured.Unstructured{Object: dvMap},
		AdditionalItems: []velero.ResourceIdentifier{},
	}, nil
}

func resetDVSpec(dv *cdiv1.DataVolume) {
	dataSourceRef := cdiv1.DataVolumeSpec{
		Source: &cdiv1.DataVolumeSource{
			Upload: &cdiv1.DataVolumeSourceUpload{},
		},
		SourceRef: &cdiv1.DataVolumeSourceRef{
			Kind:      "DataVolume",
			Name:      dv.Name,
			Namespace: &dv.Namespace,
		},
	}
	dv.Spec.Source = dataSourceRef.Source
	dv.Spec.SourceRef = dataSourceRef.SourceRef
}
