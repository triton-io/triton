package fetcher

import (
	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	"github.com/triton-io/triton/pkg/setting"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetCloneSetInCache(ns, name string, cl client.Client) (*kruiseappsv1alpha1.CloneSet, bool, error) {
	cs := &kruiseappsv1alpha1.CloneSet{}
	found, err := GetResourceInCache(ns, name, cs, cl)
	if err != nil || !found {
		cs = nil
	}

	return cs, found, err
}

func GetCloneSetsInCache(ns string, cl client.Client) ([]*kruiseappsv1alpha1.CloneSet, error) {
	cs := &kruiseappsv1alpha1.CloneSetList{}
	err := ListResourceInCache(ns, cs, cl)
	if err != nil {
		return nil, err
	}

	res := make([]*kruiseappsv1alpha1.CloneSet, 0, len(cs.Items))
	for _, c := range cs.Items {
		res = append(res, &c)
	}

	return res, err
}

func GetCloneSetInCacheByDeploy(deploy *tritonappsv1alpha1.DeployFlow, cl client.Client) (*kruiseappsv1alpha1.CloneSet, bool, error) {
	return GetCloneSetInCache(deploy.Namespace, deploy.Spec.Application.AppName, cl)
}

func GetCloneSetInCacheOwnedByDeploy(deploy *tritonappsv1alpha1.DeployFlow, cl client.Client) (*kruiseappsv1alpha1.CloneSet, bool, error) {
	cs, found, err := GetCloneSetInCache(deploy.Namespace, deploy.Spec.Application.AppName, cl)
	if cs != nil {
		if !metav1.IsControlledBy(cs, deploy) {
			cs = nil
			found = false
		}
	}
	return cs, found, err
}

func GetCloneSetInCacheByPod(pod *corev1.Pod, cl client.Client) (*kruiseappsv1alpha1.CloneSet, bool, error) {
	var cloneSetName string
	for _, o := range pod.OwnerReferences {
		if o.Kind == setting.TypeCloneSet {
			cloneSetName = o.Name
		}
	}

	return GetCloneSetInCache(pod.Namespace, cloneSetName, cl)
}
