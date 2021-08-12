package base

import (
	"fmt"
	"time"

	"github.com/triton-io/triton/pkg/kube/client"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
)

func GetPodInCache(ns, name string) (*corev1.Pod, bool, error) {
	clientset := client.GetKubeClient()
	podInformer := client.NewSharedInformerFactory(clientset, time.Second*30).Core().V1().Pods()
	podLister := podInformer.Lister()

	pod, err := podLister.Pods(ns).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if pod.GetDeletionTimestamp() != nil {
		klog.V(2).Infof("pod %s is terminating, return nil", pod.GetName())
		return nil, false, nil
	}

	return pod, true, nil
}

// GetPodsInCache gets pods in local cache, if
// 1. ns is empty, return all pods in all namespace
// 2. ns is set, but ls is nil, return all pods in namespace
// 3. both ns and ls are set, return all pods matching ls
func GetPodsInCache(ns string, ls labels.Selector) ([]*corev1.Pod, error) {
	clientset := client.GetKubeClient()
	informerFactory := client.NewSharedInformerFactory(clientset, time.Second*30)
	podInformer := informerFactory.Core().V1().Pods()
	podLister := podInformer.Lister()

	pods, err := podLister.Pods(ns).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var res []*corev1.Pod
	for _, p := range pods {
		if p.GetDeletionTimestamp().IsZero() {
			res = append(res, p)
		}
	}
	return res, nil
}

func GetReplicaSetsInCache(d *appsv1.Deployment) ([]*appsv1.ReplicaSet, error) {
	clientset := client.GetKubeClient()
	ns := d.GetNamespace()
	informerFactory := client.NewSharedInformerFactory(clientset, time.Second*30)
	rsInformer := informerFactory.Apps().V1().ReplicaSets()
	rsLister := rsInformer.Lister()

	all, err := rsLister.ReplicaSets(ns).List(labels.Set(d.Spec.Selector.MatchLabels).AsSelector())
	if err != nil {
		return nil, err
	}

	// Only include those whose ControllerRef matches the Deployment.
	owned := make([]*appsv1.ReplicaSet, 0, len(all))
	for _, rs := range all {
		if metav1.IsControlledBy(rs, d) {
			owned = append(owned, rs)
		}
	}
	return owned, nil
}

// GetNewReplicaSetWithoutRetry returns the new rs (the one with the same pod template).
func GetNewReplicaSetWithoutRetry(d *appsv1.Deployment) (*appsv1.ReplicaSet, error) {
	rsList, err := GetReplicaSetsInCache(d)
	if err != nil {
		return nil, err
	}
	if len(rsList) == 0 {
		return nil, fmt.Errorf("new replicaSet for deployment %s is not found", d.Name)
	}
	return FindNewReplicaSet(d, rsList), nil
}
