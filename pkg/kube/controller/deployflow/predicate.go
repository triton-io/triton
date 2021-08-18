package deployflow

import (
	"reflect"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	internalcloneset "github.com/triton-io/triton/pkg/kube/types/cloneset"
	"github.com/triton-io/triton/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// CloneSetStatusChangedPredicate is a predicate which focuses on CloneSet status changes.
type CloneSetStatusChangedPredicate struct {
	predicate.Funcs
}

func (CloneSetStatusChangedPredicate) Update(e event.UpdateEvent) bool {
	logger := log.WithField("event", e)

	if e.MetaOld == nil {
		logger.Error("UpdateEvent has no old metadata")
		return false
	}
	if e.ObjectOld == nil {
		logger.Error("UpdateEvent has no old runtime object to update")
		return false
	}
	if e.ObjectNew == nil {
		logger.Error("UpdateEvent has no new runtime object for update")
		return false
	}
	if e.MetaNew == nil {
		logger.Error("UpdateEvent has no new metadata")
		return false
	}

	oldCloneSet, ok := e.ObjectOld.(*kruiseappsv1alpha1.CloneSet)
	if !ok {
		logger.Error("Old runtime object is not a CloneSet")
		return false
	}
	newCloneSet, ok := e.ObjectNew.(*kruiseappsv1alpha1.CloneSet)
	if !ok {
		logger.Error("New runtime object is not a CloneSet")
		return false
	}
	if !internalcloneset.FromCloneSet(newCloneSet).Managed() {
		return false
	}

	return !reflect.DeepEqual(oldCloneSet.Status, newCloneSet.Status)
}

func (CloneSetStatusChangedPredicate) Create(e event.CreateEvent) bool {
	return false
}
