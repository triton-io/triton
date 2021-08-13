package base

import (
	"context"
	"fmt"

	"github.com/triton-io/triton/pkg/setting"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SetPodReadinessGate(ns, name string, cl client.Client) error {
	patchBytes := []byte(fmt.Sprintf(`{"status":{"conditions":[{"type":"%s", "status":"True"}]}}`, setting.PodReadinessGate))
	return patchPodStatus(ns, name, patchBytes, cl)
}

func DeletePod(ns, name string, cl client.Client) error {
	err := cl.Delete(context.TODO(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	})

	return client.IgnoreNotFound(err)
}

func patchPodStatus(ns, name string, patchBytes []byte, cl client.Client) error {
	return cl.Status().Patch(context.TODO(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}, client.RawPatch(types.StrategicMergePatchType, patchBytes))
}
