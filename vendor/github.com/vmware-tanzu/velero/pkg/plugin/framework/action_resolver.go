/*
Copyright the Velero Contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	isv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/item_snapshotter/v1"

	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
)

/*
Velero has a variety of Actions that can be executed on Kubernetes resources.  The Actions (BackupItemAction, RestoreItemAction
and others) implement the Applicable interface which returns a ResourceSelector for the Action.  The ResourceSelector
can specify namespaces, resource names and labels to include or exclude.  The ResourceSelector is resolved into lists
of namespaces and resources present in the backup to be matched against.  These lists and the label selector are then used to
decide whether or not the ResolvedAction should be used for a particular resource.
*/

// ResolvedAction is an action that has had the namespaces, resources names and labels to include or exclude resolved
type ResolvedAction interface {
	// ShouldUse returns true if the resolved namespaces, resource names and labels match those passed in the parameters.
	// metadata is optional and may be nil
	ShouldUse(groupResource schema.GroupResource, namespace string, metadata metav1.Object,
		log logrus.FieldLogger) bool
}

// resolvedAction is a core struct that holds the resolved namespaces, resource names and labels
type resolvedAction struct {
	ResourceIncludesExcludes  *collections.IncludesExcludes
	NamespaceIncludesExcludes *collections.IncludesExcludes
	Selector                  labels.Selector
}

func (recv resolvedAction) ShouldUse(groupResource schema.GroupResource, namespace string, metadata metav1.Object,
	log logrus.FieldLogger) bool {
	if !recv.ResourceIncludesExcludes.ShouldInclude(groupResource.String()) {
		log.Debug("Skipping action because it does not apply to this resource")
		return false
	}

	if namespace != "" && !recv.NamespaceIncludesExcludes.ShouldInclude(namespace) {
		log.Debug("Skipping action because it does not apply to this namespace")
		return false
	}

	if namespace == "" && !recv.NamespaceIncludesExcludes.IncludeEverything() {
		log.Debug("Skipping action because resource is cluster-scoped and action only applies to specific namespaces")
		return false
	}

	if metadata != nil && !recv.Selector.Matches(labels.Set(metadata.GetLabels())) {
		log.Debug("Skipping action because label selector does not match")
		return false
	}
	return true
}

// resolveAction resolves the resources, namespaces and selector into fully-qualified versions
func resolveAction(helper discovery.Helper, action velero.Applicable) (resources *collections.IncludesExcludes,
	namespaces *collections.IncludesExcludes, selector labels.Selector, err error) {
	resourceSelector, err := action.AppliesTo()
	if err != nil {
		return nil, nil, nil, err
	}

	resources = collections.GetResourceIncludesExcludes(helper, resourceSelector.IncludedResources, resourceSelector.ExcludedResources)
	namespaces = collections.NewIncludesExcludes().Includes(resourceSelector.IncludedNamespaces...).Excludes(resourceSelector.ExcludedNamespaces...)

	selector = labels.Everything()
	if resourceSelector.LabelSelector != "" {
		if selector, err = labels.Parse(resourceSelector.LabelSelector); err != nil {
			return nil, nil, nil, err
		}
	}

	return
}

type BackupItemResolvedAction struct {
	velero.BackupItemAction
	resolvedAction
}

func NewBackupItemActionResolver(actions []velero.BackupItemAction) BackupItemActionResolver {
	return BackupItemActionResolver{
		actions: actions,
	}
}

func NewRestoreItemActionResolver(actions []velero.RestoreItemAction) RestoreItemActionResolver {
	return RestoreItemActionResolver{
		actions: actions,
	}
}

func NewDeleteItemActionResolver(actions []velero.DeleteItemAction) DeleteItemActionResolver {
	return DeleteItemActionResolver{
		actions: actions,
	}
}

func NewItemSnapshotterResolver(actions []isv1.ItemSnapshotter) ItemSnapshotterResolver {
	return ItemSnapshotterResolver{
		actions: actions,
	}
}

type ActionResolver interface {
	ResolveAction(helper discovery.Helper, action velero.Applicable) (ResolvedAction, error)
}

type BackupItemActionResolver struct {
	actions []velero.BackupItemAction
}

func (recv BackupItemActionResolver) ResolveActions(helper discovery.Helper) ([]BackupItemResolvedAction, error) {
	var resolved []BackupItemResolvedAction
	for _, action := range recv.actions {
		resources, namespaces, selector, err := resolveAction(helper, action)
		if err != nil {
			return nil, err
		}
		res := BackupItemResolvedAction{
			BackupItemAction: action,
			resolvedAction: resolvedAction{
				ResourceIncludesExcludes:  resources,
				NamespaceIncludesExcludes: namespaces,
				Selector:                  selector,
			},
		}
		resolved = append(resolved, res)
	}
	return resolved, nil
}

type RestoreItemResolvedAction struct {
	velero.RestoreItemAction
	resolvedAction
}

type RestoreItemActionResolver struct {
	actions []velero.RestoreItemAction
}

func (recv RestoreItemActionResolver) ResolveActions(helper discovery.Helper) ([]RestoreItemResolvedAction, error) {
	var resolved []RestoreItemResolvedAction
	for _, action := range recv.actions {
		resources, namespaces, selector, err := resolveAction(helper, action)
		if err != nil {
			return nil, err
		}
		res := RestoreItemResolvedAction{
			RestoreItemAction: action,
			resolvedAction: resolvedAction{
				ResourceIncludesExcludes:  resources,
				NamespaceIncludesExcludes: namespaces,
				Selector:                  selector,
			},
		}
		resolved = append(resolved, res)
	}
	return resolved, nil
}

type DeleteItemResolvedAction struct {
	velero.DeleteItemAction
	resolvedAction
}

type DeleteItemActionResolver struct {
	actions []velero.DeleteItemAction
}

func (recv DeleteItemActionResolver) ResolveActions(helper discovery.Helper) ([]DeleteItemResolvedAction, error) {
	var resolved []DeleteItemResolvedAction
	for _, action := range recv.actions {
		resources, namespaces, selector, err := resolveAction(helper, action)
		if err != nil {
			return nil, err
		}
		res := DeleteItemResolvedAction{
			DeleteItemAction: action,
			resolvedAction: resolvedAction{
				ResourceIncludesExcludes:  resources,
				NamespaceIncludesExcludes: namespaces,
				Selector:                  selector,
			},
		}
		resolved = append(resolved, res)
	}
	return resolved, nil
}

type ItemSnapshotterResolvedAction struct {
	isv1.ItemSnapshotter
	resolvedAction
}

type ItemSnapshotterResolver struct {
	actions []isv1.ItemSnapshotter
}

func (recv ItemSnapshotterResolver) ResolveActions(helper discovery.Helper) ([]ItemSnapshotterResolvedAction, error) {
	var resolved []ItemSnapshotterResolvedAction
	for _, action := range recv.actions {
		resources, namespaces, selector, err := resolveAction(helper, action)
		if err != nil {
			return nil, err
		}
		res := ItemSnapshotterResolvedAction{
			ItemSnapshotter: action,
			resolvedAction: resolvedAction{
				ResourceIncludesExcludes:  resources,
				NamespaceIncludesExcludes: namespaces,
				Selector:                  selector,
			},
		}
		resolved = append(resolved, res)
	}
	return resolved, nil
}
