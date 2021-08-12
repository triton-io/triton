/*
Copyright 2021 The Triton Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deployflow

import (
	"context"
	"flag"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	tritonappsv1alpha1 "github.com/triton-io/triton/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	flag.IntVar(&concurrentReconciles, "deployflow-workers", concurrentReconciles, "Max concurrent workers for deployflow controller.")
}

var (
	concurrentReconciles = 3
)

// Add creates a new DeployFlow Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// DeployFlowReconciler reconciles a DeployFlow object
type DeployFlowReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	reader        client.Reader
	reconcileFunc func(ctx context.Context, request reconcile.Request) (reconcile.Result, error)

	recorder record.EventRecorder
}

func newReconciler(mgr ctrl.Manager) reconcile.Reconciler {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &tritonappsv1alpha1.DeployFlow{}, "spec.Application.AppName", func(rawObj runtime.Object) []string {
		d, ok := rawObj.(*tritonappsv1alpha1.DeployFlow)
		if !ok {
			klog.Errorf("failed to get deployflow resource")
			return nil
		}
		return []string{d.Spec.Application.AppName}
	}); err != nil {
		klog.Errorf("failed to get filed indexer")
		return nil
	}

	recorder := mgr.GetEventRecorderFor("DeployFlow Controller")

	reconciler := &DeployFlowReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		reader:   mgr.GetAPIReader(),
		recorder: recorder,
	}
	reconciler.reconcileFunc = reconciler.doReconcile

	return reconciler
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {

	err := ctrl.NewControllerManagedBy(mgr).
		For(&tritonappsv1alpha1.DeployFlow{}).
		Owns(&kruiseappsv1alpha1.CloneSet{}).
		//Watches(&source.Kind{Type: &kruiseappsv1alpha1.CloneSet{}}, &handler.EnqueueRequestForOwner{}, builder.WithPredicates(CloneSetStatusChangedPredicate{})).
		Complete(r)

	if err != nil {
		return err
	}

	klog.Info("DeployFlow Controller created")

	return nil
}

var _ reconcile.Reconciler = &DeployFlowReconciler{}

//+kubebuilder:rbac:groups=apps.triton.io,resources=deployflows,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.triton.io,resources=deployflows/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.triton.io,resources=deployflows/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the DeployFlow object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile

// Reconcile reads that state of the cluster for a CloneSet object and makes changes based on the state read
// and what is in the CloneSet.Spec
func (r *DeployFlowReconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	return r.reconcileFunc(context.TODO(), req)
}

func (r *DeployFlowReconciler) doReconcile(ctx context.Context, req reconcile.Request) (res reconcile.Result, retErr error) {
	_ = log.FromContext(ctx)

	// your logic here

	return ctrl.Result{}, nil
}
