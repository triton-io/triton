package fetcher

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetConfigMapInCache(ns, name string, cl client.Client) (*corev1.ConfigMap, bool, error) {
	cm := &corev1.ConfigMap{}
	found, err := GetResourceInCache(ns, name, cm, cl)
	if err != nil || !found {
		cm = nil
	}

	return cm, found, err
}
