package watcher

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ActionReconciler struct {
	client.Client
	Action ActionFunc
	Type   runtime.Object
	Done   func()
	isDone bool

	Logger *logrus.Entry
}

func (r *ActionReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	logger := r.Logger.WithField("object", req.NamespacedName)

	if r.isDone {
		return ctrl.Result{}, nil
	}

	obj := r.Type.DeepCopyObject()
	if err := r.Get(context.TODO(), req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("object not found, stop reconciler")
			r.done()
			return reconcile.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	meta, ok := obj.(metav1.Object)
	if !ok {
		logger.Error("metadata is missing")
		return ctrl.Result{}, fmt.Errorf("metadata is missing")
	}

	if !meta.GetDeletionTimestamp().IsZero() {
		logger.Info("object terminating, stop reconciler")
		r.done()
		return reconcile.Result{}, nil
	}

	done, err := r.Action(obj)
	if err != nil || done {
		r.done()
	}

	return reconcile.Result{}, nil
}

func (r *ActionReconciler) done() {
	r.Done()
	r.isDone = true
}

func (r *ActionReconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}
