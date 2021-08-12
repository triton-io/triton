package base

import (
	"sort"

	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

// EqualIgnoreHash returns true if two given podTemplateSpec are equal,
// ignoring the diff in value of Labels[pod-template-hash]
// We ignore pod-template-hash because:
// 1. The hash result would be different upon podTemplateSpec API changes
//    (e.g. the addition of a new field will cause the hash code to change)
// 2. The deployment template won't have hash labels
func EqualIgnoreHash(template1, template2 *corev1.PodTemplateSpec) bool {
	t1Copy := template1.DeepCopy()
	t2Copy := template2.DeepCopy()
	// Remove hash labels from template.Labels before comparing
	delete(t1Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	delete(t2Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	return apiequality.Semantic.DeepEqual(t1Copy, t2Copy)
}

// ReplicaSetsByCreationTimestamp sorts a list of ReplicaSet by creation timestamp, using their names as a tie breaker.
type ReplicaSetsByCreationTimestamp []*appsv1.ReplicaSet

func (o ReplicaSetsByCreationTimestamp) Len() int      { return len(o) }
func (o ReplicaSetsByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o ReplicaSetsByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}

// ObjectsByCreationTimestamp sorts a list of Pod by creation timestamp, using their names as a tie breaker.
type PodsByCreationTimestamp []*corev1.Pod

func (o PodsByCreationTimestamp) Len() int      { return len(o) }
func (o PodsByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o PodsByCreationTimestamp) Less(i, j int) bool {
	// we use "after" here since we want to the newest one comes first
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return !o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}

// DeploysByCreationTimestamp sorts a list of Deploy by creation timestamp, using their names as a tie breaker.
type DeploysByCreationTimestamp []*tritonappsv1alpha1.DeployFlow

func (o DeploysByCreationTimestamp) Len() int      { return len(o) }
func (o DeploysByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o DeploysByCreationTimestamp) Less(i, j int) bool {
	// we use "after" here since we want to the newest one comes first
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return !o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}

// FindNewReplicaSet returns the new RS this given deployment targets (the one with the same pod template).
func FindNewReplicaSet(d *appsv1.Deployment, rsList []*appsv1.ReplicaSet) *appsv1.ReplicaSet {
	sort.Sort(ReplicaSetsByCreationTimestamp(rsList))
	for i := range rsList {
		if EqualIgnoreHash(&rsList[i].Spec.Template, &d.Spec.Template) {
			// In rare cases, such as after cluster upgrades, Deployment may end up with
			// having more than one new ReplicaSets that have the same template as its template,
			// see https://github.com/kubernetes/kubernetes/issues/40415
			// We deterministically choose the oldest new ReplicaSet.
			return rsList[i]
		}
	}
	// new ReplicaSet does not exist.
	return nil
}
