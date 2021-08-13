package base

import (
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func InterestedNamespaces() sets.String {
	return sets.NewString(viper.GetStringSlice("controller.managed_namespaces")...)
}

// NamespacePredicate is a predicate which focuses on interested namespaces event
type NamespacePredicate struct{}

var _ predicate.Predicate = NamespacePredicate{}

func (NamespacePredicate) Update(e event.UpdateEvent) bool {
	return match(e.MetaNew)
}

func (NamespacePredicate) Delete(e event.DeleteEvent) bool {
	return match(e.Meta)
}

func (NamespacePredicate) Generic(e event.GenericEvent) bool {
	return match(e.Meta)
}

func (NamespacePredicate) Create(e event.CreateEvent) bool {

	return match(e.Meta)
}

func match(obj metav1.Object) bool {
	if obj == nil {
		return false
	}
	return InterestedNamespaces().Has(obj.GetNamespace())
}
