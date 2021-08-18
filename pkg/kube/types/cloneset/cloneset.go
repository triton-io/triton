package cloneset

import (
	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/triton-io/triton/pkg/kube/types/workload"
	"github.com/triton-io/triton/pkg/setting"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// CloneSet is the wrapper for kruiseappsv1alpha1.CloneSet type.
type CloneSet struct {
	*kruiseappsv1alpha1.CloneSet
	*workload.Base
	appID, groupID int
	appName        string
}

var _ workload.Interface = &CloneSet{}

func FromCloneSet(cs *kruiseappsv1alpha1.CloneSet) *CloneSet {
	if cs == nil {
		return nil
	}

	var appID, groupID int
	appName := ""

	c := &CloneSet{
		CloneSet: cs,
		appID:    appID,
		groupID:  groupID,
		appName:  appName,
	}
	c.Base = &workload.Base{Interface: c}
	return c
}

// Unwrap returns the appsv1.CloneSet object.
func (cs *CloneSet) Unwrap() *kruiseappsv1alpha1.CloneSet {
	return cs.CloneSet
}

func (cs *CloneSet) GetAppID() int {
	return cs.appID
}

func (cs *CloneSet) GetGroupID() int {
	return cs.groupID
}

func (cs *CloneSet) GetAppName() string {
	return cs.appName
}

func (cs *CloneSet) GetPodLabels() labels.Set {
	return cs.CloneSet.Spec.Template.GetLabels()
}

func (cs *CloneSet) GetUpdateRevision() string {
	ur := cs.Status.UpdateRevision
	if ur == "" {
		return ""
	}
	return ur[strings.LastIndex(ur, "-")+1:]

}

func (cs *CloneSet) GetPhase() string {
	if cs.ready() {
		return setting.DeploymentReady
	}
	return setting.DeploymentInProgress
}

func (cs *CloneSet) GetContainers() []corev1.Container {
	return cs.Spec.Template.Spec.Containers
}

func (cs *CloneSet) SetAppContainer(c *corev1.Container) {
	containerName := cs.GetAppContainerName()
	cts := cs.Spec.Template.Spec.Containers
	updated := []corev1.Container{*c}
	if len(cts) > 1 {
		for _, c := range cts {
			if c.Name != containerName {
				updated = append(updated, c)
			}
		}
	}

	cs.CloneSet.Spec.Template.Spec.Containers = updated
}

func (cs *CloneSet) SetReplicas(replicas *int32) {
	cs.CloneSet.Spec.Replicas = replicas
}

func (cs *CloneSet) SetAppName(name string) {
	ls := cs.GetLabels()
	if ls == nil {
		ls = make(map[string]string)
	}
	ls[setting.AppLabel] = name

	cs.SetLabels(ls)
}

func (cs *CloneSet) SetPodAppName(name string) {
	ls := cs.GetPodLabels()
	if ls == nil {
		ls = make(map[string]string)
	}
	ls[setting.AppLabel] = name

	cs.SetPodLabels(ls)
}

func (cs *CloneSet) SetLabels(ls labels.Set) {
	cs.CloneSet.SetLabels(ls)
}

func (cs *CloneSet) SetPodLabels(ls labels.Set) {
	cs.CloneSet.Spec.Template.SetLabels(ls)
}

func (cs *CloneSet) ready() bool {
	return *cs.Spec.Replicas == cs.Status.UpdatedReadyReplicas
}

func (cs *CloneSet) Managed() bool {
	managedSelector := cs.GetManagedSelector()
	return managedSelector.Matches(labels.Set(cs.GetLabels()))
}

func CloneSetStatusSynced(deploy *tritonappsv1alpha1.DeployFlow, cl client.Client) bool {
	cs, found, err := fetcher.GetCloneSetInCacheOwnedByDeploy(deploy, cl)
	if err != nil || !found {
		return false
	}

	return cs.GetGeneration() == cs.Status.ObservedGeneration
}
