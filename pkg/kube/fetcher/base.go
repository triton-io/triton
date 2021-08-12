package fetcher

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetResourceInCache(ns, name string, obj runtime.Object, cl client.Client) (bool, error) {
	err := cl.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: name}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func ListResourceInCache(ns string, obj runtime.Object, cl client.Client) error {
	return cl.List(context.TODO(), obj, client.InNamespace(ns))
}
