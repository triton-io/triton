package fetcher

import (
	"context"
	"sort"
	"time"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	tritonappsv1alpha1 "github.com/triton-io/triton/api/v1alpha1"
	"github.com/triton-io/triton/pkg/services/base"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetDeployInCache(ns, name string, cl client.Client) (*tritonappsv1alpha1.DeployFlow, bool, error) {
	d := &tritonappsv1alpha1.DeployFlow{}
	found, err := GetResourceInCache(ns, name, d, cl)
	if err != nil || !found {
		d = nil
	}

	return d, found, err
}

func GetDeployInCacheOwningCloneSet(cs *kruiseappsv1alpha1.CloneSet, cl client.Client) (*tritonappsv1alpha1.DeployFlow, bool, error) {
	deployRef := metav1.GetControllerOf(cs)
	if deployRef == nil {
		return nil, false, nil
	}

	deploy := &tritonappsv1alpha1.DeployFlow{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Namespace: cs.Namespace, Name: deployRef.Name}, deploy); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return deploy, true, nil
}

type DeployFilter struct {
	Namespace    string
	InstanceName string
	Start        int
	PageSize     int
	After        *time.Time
}

func GetDeploysInCache(f DeployFilter, cl client.Client) ([]*tritonappsv1alpha1.DeployFlow, error) {
	max := 10000
	if f.PageSize > max {
		max = f.PageSize
	}

	opts := []client.ListOption{client.InNamespace(f.Namespace), client.Limit(max)}
	if f.InstanceName != "" {
		//cs, found, err := base.GetCloneSetInCache(ns, instanceName)
		//if err != nil {
		//	return nil, err
		//} else if !found {
		//	return nil, nil
		//}
		//opts = append(opts, client.MatchingLabels(cs.Spec.Selector.MatchLabels))
		opts = append(opts, client.MatchingFields{"spec.instanceName": f.InstanceName})
	}

	d := &tritonappsv1alpha1.DeployFlowList{}
	err := cl.List(context.TODO(), d, opts...)
	if err != nil {
		return nil, err
	}

	total := len(d.Items)
	deployList := make([]*tritonappsv1alpha1.DeployFlow, 0, total)
	for i, dpl := range d.Items {
		updatedTime := dpl.Status.UpdatedAt
		if updatedTime.IsZero() {
			updatedTime = dpl.CreationTimestamp
		}
		if f.After != nil && updatedTime.Time.Before(*f.After) {
			continue
		}
		deployList = append(deployList, &d.Items[i])
	}
	total = len(deployList)

	sort.Sort(base.DeploysByCreationTimestamp(deployList))

	if f.Start >= total {
		deployList = nil
	} else if f.Start+f.PageSize >= total {
		deployList = deployList[f.Start:]
	} else {
		deployList = deployList[f.Start : f.Start+f.PageSize]
	}

	return deployList, nil
}

func GetDeployFromAPIServer(ns, name string, reader client.Reader) (*tritonappsv1alpha1.DeployFlow, error) {
	d := &tritonappsv1alpha1.DeployFlow{}
	err := reader.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: name}, d)

	return d, err
}
