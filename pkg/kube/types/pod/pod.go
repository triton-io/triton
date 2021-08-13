package pod

import (
	"fmt"
	"strings"

	"github.com/triton-io/triton/pkg/kube/types/workload"
	"github.com/triton-io/triton/pkg/setting"
	"github.com/triton-io/triton/pkg/utils/strconv"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// Pod is the wrapper for corev1.Pod type.
type Pod struct {
	*corev1.Pod
	*workload.Base
}

var _ workload.Interface = &Pod{}

func FromPod(p *corev1.Pod) *Pod {
	if p == nil {
		return nil
	}

	pod := &Pod{
		Pod: p,
	}
	pod.Base = &workload.Base{Interface: pod}

	return pod
}

// Unwrap returns the appsv1.Pod object.
func (p *Pod) Unwrap() *corev1.Pod {
	return p.Pod
}

func (p *Pod) GetUpdateRevision() string {
	l := p.GetLabels()
	for k, v := range l {
		if k == appsv1.ControllerRevisionHashLabelKey {
			return v[strings.LastIndex(v, "-")+1:]
		}
	}
	return ""
}

func (p *Pod) GetHostIP() string {
	return p.Status.HostIP
}

func (p *Pod) GetPodIP() string {
	return p.Status.PodIP
}

func (p *Pod) String() string {
	return fmt.Sprintf("%s/%s", p.GetNamespace(), p.GetName())
}

func (p *Pod) GetPhase() corev1.PodPhase {
	if p.Ready() {
		return setting.PodReady
	}

	if p.ContainersReady() {
		return setting.ContainersReady
	}

	if p.Failed() {
		return setting.PodFailed
	}

	return p.Status.Phase
}

func (p *Pod) Ready() bool {
	cs := p.Status.Conditions
	for _, c := range cs {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// For a Pod that uses custom conditions, that Pod is evaluated to be ready only when both the following statements apply:
// All containers in the Pod are ready.
// All conditions specified in readinessGates are True.
// When a Pod's containers are Ready but at least one custom condition is missing or False, the kubelet sets the Pod's condition to ContainersReady.
// See details here: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-readiness-status
func (p *Pod) ContainersReady() bool {
	cs := p.Status.Conditions
	for _, c := range cs {
		if c.Type == corev1.ContainersReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (p *Pod) Failed() bool {
	if p.Ready() {
		return false
	}

	return p.Restarted()
}

func (p *Pod) Restarted() bool {
	for _, cs := range p.Status.ContainerStatuses {
		if cs.RestartCount > 0 {
			return true
		}
	}
	return false
}

func (p *Pod) GetAppID() int {
	podLabels := p.GetLabels()
	if appID, ok := podLabels[setting.AppIDLabel]; ok {
		return strconv.MustAtoi(appID)
	}
	return 0
}

func (p *Pod) GetGroupID() int {
	podLabels := p.GetLabels()
	if groupID, ok := podLabels[setting.GroupIDLabel]; ok {
		return strconv.MustAtoi(groupID)
	}
	return 0
}

func (p *Pod) GetInstanceName() string {
	podLabels := p.GetLabels()
	if ins, ok := podLabels[setting.AppInstanceLabel]; ok {
		return ins
	}
	return ""
}

func (p *Pod) GetContainers() []corev1.Container {
	return p.Spec.Containers
}

func (p *Pod) PodReady() bool {
	for _, c := range p.Status.Conditions {
		if c.Type == setting.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
