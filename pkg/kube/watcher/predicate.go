package watcher

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// singleObjectPredicate is a predicate which focuses on single object.
type singleObjectPredicate struct {
	Namespace, Name string
}

var _ predicate.Predicate = singleObjectPredicate{}

func (p singleObjectPredicate) Update(e event.UpdateEvent) bool {
	metaNew := e.MetaNew
	metaOld := e.MetaOld
	if metaNew == nil {
		return false
	}
	if metaOld != nil {
		if metaOld.GetResourceVersion() == metaNew.GetResourceVersion() {
			return false
		}
	}

	return match(e.MetaNew, p.Namespace, p.Name)
}

func (p singleObjectPredicate) Delete(e event.DeleteEvent) bool {
	return match(e.Meta, p.Namespace, p.Name)
}

func (p singleObjectPredicate) Generic(e event.GenericEvent) bool {
	return match(e.Meta, p.Namespace, p.Name)
}

func (p singleObjectPredicate) Create(e event.CreateEvent) bool {
	return match(e.Meta, p.Namespace, p.Name)
}

func match(obj metav1.Object, ns, name string) bool {
	if obj == nil {
		return false
	}
	return obj.GetNamespace() == ns && obj.GetName() == name
}
