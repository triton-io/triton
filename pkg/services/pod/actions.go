package pod

import (
	"sort"

	fetcher "github.com/triton-io/triton/pkg/kube/fetcher"
	"github.com/triton-io/triton/pkg/services/base"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Filter struct {
	Namespace    string
	InstanceName string
	IP           string
}

func (f *Filter) LabelSelector(cl client.Client) labels.Selector {
	if f.InstanceName != "" {
		cs, found, err := fetcher.GetCloneSetInCache(f.Namespace, f.InstanceName, cl)
		if err != nil || !found {
			return nil
		}
		return labels.Set(cs.Spec.Selector.MatchLabels).AsSelector()
	}

	return labels.Everything()
}

func FetchPods(f *Filter, ls labels.Selector, noSort bool) ([]*corev1.Pod, error) {
	if ls == nil {
		return nil, nil
	}

	pods, err := base.GetPodsInCache(f.Namespace, ls)
	if err != nil {
		return nil, err
	}

	if !noSort {
		sort.Sort(base.PodsByCreationTimestamp(pods))
	}

	if f.IP == "" {
		return pods, nil
	}

	for _, pod := range pods {
		if pod.Status.PodIP == f.IP {
			return []*corev1.Pod{pod}, nil
		}
	}

	return nil, nil
}
