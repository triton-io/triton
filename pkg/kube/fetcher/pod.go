package fetcher

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetPodInCache(ns, name string, cl client.Client) (*corev1.Pod, bool, error) {
	p := &corev1.Pod{}
	found, err := GetResourceInCache(ns, name, p, cl)
	if err != nil || !found {
		p = nil
	}

	return p, found, err
}
