package base

import (
	"github.com/triton-io/triton/pkg/kube/fetcher"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
)

func CloneSetStatusSynced(deploy *tritonappsv1alpha1.DeployFlow, cl client.Client) bool {
	cs, found, err := fetcher.GetCloneSetInCacheOwnedByDeploy(deploy, cl)
	if err != nil || !found {
		return false
	}

	return cs.GetGeneration() == cs.Status.ObservedGeneration
}
