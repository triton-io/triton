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
	"encoding/json"
	"flag"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	terrors "github.com/triton-io/triton/pkg/errors"
	"github.com/triton-io/triton/pkg/kube/controllers/base"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	internaldeploy "github.com/triton-io/triton/pkg/kube/types/deploy"
	"github.com/triton-io/triton/pkg/services/deployflow"
	"github.com/triton-io/triton/pkg/setting"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/sirupsen/logrus"
	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	"github.com/triton-io/triton/pkg/log"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	logger        *logrus.Entry
	reconcileFunc func(ctx context.Context, request reconcile.Request) (reconcile.Result, error)

	recorder record.EventRecorder
}

func newReconciler(mgr ctrl.Manager) reconcile.Reconciler {
	logger := log.WithField("controller", "DeployFlow")
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &tritonappsv1alpha1.DeployFlow{}, "spec.Application.AppName", func(rawObj runtime.Object) []string {
		d, ok := rawObj.(*tritonappsv1alpha1.DeployFlow)
		if !ok {
			logger.Errorf("failed to get deployflow resource")
			return nil
		}
		return []string{d.Spec.Application.AppName}
	}); err != nil {
		logger.Errorf("failed to get filed indexer")
		return nil
	}

	recorder := mgr.GetEventRecorderFor("DeployFlow Controller")

	reconciler := &DeployFlowReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		reader:   mgr.GetAPIReader(),
		logger:   logger,
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

	log.Info("DeployFlow Controller created")

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

	// your logic here
	logger := r.logger.WithField("deploy", req.NamespacedName)

	deploy := &tritonappsv1alpha1.DeployFlow{}

	// get deploy from API Server instead of cache
	if err := r.reader.Get(ctx, req.NamespacedName, deploy); err != nil {
		logger.WithError(err).Error("unable to fetch Deploy")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	idl := internaldeploy.FromDeploy(deploy)
	if idl.Finished() {
		// If a deploy is finished, never touch it again
		return ctrl.Result{}, nil
	}

	// logger.Info("Start to reconcile")

	status := *deploy.Status.DeepCopy()
	defer func() {
		if !reflect.DeepEqual(status, idl.Status) {
			if err := r.updateDeployStatus(idl); err != nil {
				logger.WithError(err).Error("unable to update Deploy status")
			}
		}
	}()

	defer func() {
		if idl.Finished() {
			r.postDeploy(idl)
		}
		if err := r.DeleteCloneSetWhenActionIsScaleInZero(deploy); err != nil {
			logger.WithError(err).Error("unable delete cloneSet which replicas is 0 when action is scale in")
		}
	}()

	if err := r.process(idl); err != nil {
		if e, ok := err.(terrors.RequeueError); ok {
			return ctrl.Result{RequeueAfter: e.RequeueAfter()}, nil
		}
		logger.WithError(err).Error("failed to process deploy")

		r.recorder.Event(idl.Unwrap(), corev1.EventTypeWarning, eventReasonUnhealthy, err.Error())
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DeployFlowReconciler) updateDeployStatus(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	now := metav1.Now()
	logger.Infof("Updating deploy status at %s", now)
	idl.SetUpdatedAt(now)

	ctx := context.Background()
	if err := r.Status().Update(ctx, idl.Unwrap()); err != nil {
		if !apierrors.IsConflict(err) {
			return errors.Wrap(err, "failed to update Deploy status")
		}

		logger.Warn("can not update deploy status due to a conflict, fetch latest version and try again")
		updated := &tritonappsv1alpha1.DeployFlow{}
		if err := r.reader.Get(ctx, types.NamespacedName{Namespace: idl.Namespace, Name: idl.Name}, updated); err != nil {
			return errors.Wrap(err, "failed to fetch Deploy from API Server")
		}

		if idl.CurrentBatchSmoking() || !idl.CurrentBatchIsPodReady() {
			// save the latest pod status updated by CloneSet controller
			populatedPods := updated.Status.Pods
			currentBatchInfo := internaldeploy.FromDeploy(updated).CurrentBatchInfo()
			currentBatch := currentBatchInfo.Batch
			currentBatchPods := currentBatchInfo.Pods
			failedReplicas := currentBatchInfo.FailedReplicas

			updated.Status = idl.Status

			// restore the latest pod status
			updated.Status.Pods = populatedPods
			if len(currentBatchPods) > 0 {
				updated.Status.Conditions[currentBatch-1].Pods = currentBatchPods
				updated.Status.Conditions[currentBatch-1].FailedReplicas = failedReplicas
			}

		}

		//if idl.CurrentBatchPhase() == tritonappsv1alpha1.BatchSmoking {
		//	err := mergeDeployStatus(idl.Unwrap(), updated, logger)
		//	if err != nil {
		//		return errors.Wrap(err, "failed to merge Deploy status")
		//	}
		//}
		//
		//idl.SetResourceVersion(updated.ResourceVersion)

		data, _ := json.Marshal(updated.Status)
		logger.Debugf("the current status is %s", data)
		return r.Status().Update(ctx, updated)
	}

	return nil
}

func (r *DeployFlowReconciler) process(idl *internaldeploy.Deploy) error {
	if err := r.processPausedOrCanceledDeploy(idl); err != nil {
		return err
	}

	switch idl.Status.Phase {
	case "":
		return r.processNewDeploy(idl)
	case tritonappsv1alpha1.Pending:
		if !idl.Paused() {
			return r.processPendingDeploy(idl)
		}
	case tritonappsv1alpha1.Initializing:
		return r.processInitializingDeploy(idl)
	case tritonappsv1alpha1.BatchStarted:
		if !idl.Paused() {
			return r.processBatch(idl)
		}
	case tritonappsv1alpha1.BatchFinished:
		return r.processBatchFinishedDeploy(idl)
	}

	return nil
}

func (r *DeployFlowReconciler) processNewDeploy(idl *internaldeploy.Deploy) error {
	esLogger := r.logger.WithField("deploy", idl)
	esLogger.Info("Start to process new deploy")
	idl.PrepareNewDeploy()

	return nil
}

func (r *DeployFlowReconciler) processPendingDeploy(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	cs, found, err := fetcher.GetCloneSetInCacheByDeploy(idl.Unwrap(), r.Client)
	if err != nil {
		logger.WithError(err).Errorf("failed to fetch cloneSet %s", idl.Spec.Application.InstanceName)
		return err
	} else if !found {
		if idl.Spec.Action != setting.Create {
			idl.MarkAsFailed()
			return nil
		}
		logger.Infof("cloneSet %s does not exist, start to create it", idl.Spec.Application.InstanceName)
		err := r.createCloneSet(idl)
		if err != nil {
			logger.WithError(err).Error("failed to create cloneSet")
			return err
		}
		logger.Infof("cloneSet created, start to process the deploy")
	} else {
		if idl.Spec.Action == setting.Create {
			idl.MarkAsFailed()
			return nil
		}
		finished, paused, err := lastDeployFinishedOrPaused(cs, r.Client)
		if err != nil {
			logger.WithError(err).Error("failed to check last deploy status")
			return err
		}

		if !finished {
			// if last deploy is not paused, never move forward,
			// if it is paused, move forward if it is a update.
			if !paused || !idl.RevisionChanged() {
				logger.Info("last deploy in progress, sleep 5 seconds and try again")
				return terrors.NewLastDeployInProgressError(5 * time.Second)
			}

			logger.Info("last deploy is paused, abort it.")
			if err := abortPausedDeploy(cs, r.Client); err != nil {
				logger.WithError(err).Error("Failed to update deploy status")
				return err
			}
		}

		if idl.RevisionChanged() {
			logger.Infof("cloneSet %s is found, start to update it", idl.Spec.Application.InstanceName)
			if err := r.updateCloneSet(idl); err != nil {
				logger.WithError(err).Error("Failed to update cloneSet")
				return err
			}
		} else {
			logger.Infof("Taking ownership of cloneSet %s", idl.Spec.Application.InstanceName)
			if err := r.takeOwnershipOfCloneSet(idl); err != nil {
				logger.WithError(err).Errorf("Failed to set owner for cloneSet %s", idl.Spec.Application.InstanceName)
				return err
			}
		}
	}

	logger.Infof("Resource updated")
	idl.StartProgress()

	return nil
}

func lastDeployFinishedOrPaused(cs *kruiseappsv1alpha1.CloneSet, cl client.Client) (finished, paused bool, err error) {
	deploy, found, err := fetcher.GetDeployInCacheOwningCloneSet(cs, cl)
	if err != nil || !found {
		return true, false, err
	}

	idl := internaldeploy.FromDeploy(deploy)
	return idl.Finished(), idl.Paused(), nil
}

func abortPausedDeploy(cs *kruiseappsv1alpha1.CloneSet, cl client.Client) error {
	deploy, _, err := fetcher.GetDeployInCacheOwningCloneSet(cs, cl)
	if err != nil {
		return err
	}
	if deploy == nil {
		return nil
	}

	idl := internaldeploy.FromDeploy(deploy)
	idl.MarkAsAborted()

	// use update instead of patch here to make sure deploy is not changed yet.
	return cl.Status().Update(context.TODO(), idl.Unwrap())
}

func (r *DeployFlowReconciler) processPausedOrCanceledDeploy(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	if !idl.Interruptible() {
		return nil
	}

	// pause the CloneSet when deploy is canceled.
	if idl.ShouldCancel() {
		//TODO if cancel batch is canary, rollback cloneset
		logger.Info("Deploy canceled, pause the CloneSet")
		if err := r.pauseCloneSet(idl); err != nil {
			logger.WithError(err).Error("Failed to pause CloneSet")
			return err
		}

		logger.Info("Deploy canceled")
		idl.MarkAsCanceled()
		err := r.ResetReplicasAfterCanceled(idl)
		if err != nil {
			logger.WithError(err).Error("Failed to reset replicas of cloneset")
			return err
		}
		return nil
	}

	paused := idl.DesiredPausedState()
	if paused != nil && *paused != idl.Paused() {

		if !*paused && idl.CurrentBatchFailed() {
			logger.Warn("There are failure pods in current batch, you need to fix them before resuming the Deploy")
			return nil
		}

		if err := r.pauseOrResumeCloneSet(idl, *paused); err != nil {
			logger.WithError(err).Error("Failed to pause or resume CloneSet")
			return err
		}

		logger.Infof("Deploy paused is %t", *paused)
		idl.SetPaused(*paused)

	}

	return nil
}

func (r *DeployFlowReconciler) ResetReplicasAfterCanceled(idl *internaldeploy.Deploy) error {
	if idl.Spec.Action == setting.Create {
		return nil
	}
	replicas := int(*idl.Spec.Application.Replicas)
	patchBytes := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	return PatchCloneSet(idl.Unwrap(), patchBytes, r.Client)
}

func (r *DeployFlowReconciler) processInitializingDeploy(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	// do not move forward until fields like updateRevision, updatedReplicas...
	// in .status is updated
	if !CloneSetStatusSynced(idl.Unwrap(), r.Client) {
		logger.Info("CloneSet status is not updated yet, skip and wait for next try.")

		return nil
	}

	if err := r.populateReplicasStatus(idl); err != nil {
		return err
	}

	logger.Info("Deploy initialized")
	idl.StartBatch()

	return nil
}

func CloneSetStatusSynced(deploy *tritonappsv1alpha1.DeployFlow, cl client.Client) bool {
	cs, found, err := fetcher.GetCloneSetInCacheOwnedByDeploy(deploy, cl)
	if err != nil || !found {
		return false
	}

	return cs.GetGeneration() == cs.Status.ObservedGeneration
}

func (r *DeployFlowReconciler) populateReplicasStatus(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	cs, found, err := fetcher.GetCloneSetInCacheOwnedByDeploy(idl.Unwrap(), r.Client)
	if err != nil || !found {
		logger.WithError(err).Error("unable to fetch CloneSet")
		return fmt.Errorf("unable to fetch CloneSet: %w", err)
	}

	idl.DeployFlow.Status.AvailableReplicas = cs.Status.AvailableReplicas
	idl.DeployFlow.Status.UpdatedReadyReplicas = cs.Status.UpdatedReadyReplicas
	idl.DeployFlow.Status.UpdatedReplicas = cs.Status.UpdatedReplicas
	idl.DeployFlow.Status.Replicas = cs.Status.Replicas
	idl.DeployFlow.Status.UpdateRevision = cs.Status.UpdateRevision

	return nil
}

func (r *DeployFlowReconciler) processBatch(idl *internaldeploy.Deploy) error {
	if err := r.populateReplicasStatus(idl); err != nil {
		return err
	}

	switch idl.CurrentBatchPhase() {
	case tritonappsv1alpha1.BatchPending:
		// the first pending batch should be processed no matter MoveForward is true or false
		if idl.CurrentBatchNumber() == 1 || idl.MoveForward() {
			return r.processNewBatch(idl)
		}
	case tritonappsv1alpha1.BatchSmoking:
		return r.processSmokingBatch(idl)
	case tritonappsv1alpha1.BatchSmoked:
		// move forward if:
		// 1. it is not a canary
		// 2. it is a canary and MoveForward is true
		if !idl.CurrentBatchIsCanary() || idl.MoveForward() {
			return r.processSmokedBatch(idl)
		}
	case tritonappsv1alpha1.BatchBaking:
		// move forward if:
		// 1. it is not a canary
		// 2. it is a canary and MoveForward is true
		if !idl.CurrentBatchIsCanary() || idl.MoveForward() {
			return r.processBakingBatch(idl)
		}
	case tritonappsv1alpha1.BatchBaked:
		return r.processFinishedBatch(idl)
	case "":
		return r.processEmptyBatch(idl)
	}

	return nil
}

func (r *DeployFlowReconciler) processBatchFinishedDeploy(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	if err := r.populateReplicasStatus(idl); err != nil {
		return err
	}

	if idl.ShouldCancel() {
		idl.MarkAsCanceled()
	}

	// if a deployflow has no change and all pods are in ContainersReady status, mark as success directly
	if idl.NoChangedDeploy() && len(idl.GetNotContainerReadyPods()) == 0 {
		logger.Info("All batches are finished, the deploy is done")
		idl.MarkAsSuccess()
		r.recorder.Event(idl.Unwrap(), corev1.EventTypeNormal, eventReasonDeployed, eventMessageDeployed)
		return nil
	}

	// if a deployfolw has no change and has pod not ready, patch the cloneset use scale strategy
	if idl.NoChangedDeploy() && len(idl.GetNotContainerReadyPods()) != 0 {
		// update cloneset to
		pods := idl.GetNotContainerReadyPods()
		patchBytes := []byte(fmt.Sprintf(`{"spec":{"scaleStrategy":{"podsToDelete":%v}}}`, pods))
		err := PatchCloneSet(idl.Unwrap(), patchBytes, r.Client)
		if err != nil {
			logger.Errorf("patch cloneset %v failed, error is %v", idl.Unwrap().Spec.Application.InstanceName, err)
			return err
		}

		statusBytes := []byte(fmt.Sprintf(`{"status":{"phase":"%s"}`, tritonappsv1alpha1.Initializing))
		_, err = deployflow.PatchDeployStatus(idl.Namespace, idl.Name, r.reader, r.Client, statusBytes)
		if err != nil {
			logger.Errorf("patch deployflow %v failed, error is %v", idl.Name, err)
			return err
		}
	}

	if idl.Status.AvailableReplicas != *idl.Spec.Application.Replicas ||
		*idl.Spec.Application.Replicas != idl.Status.Replicas ||
		*idl.Spec.Application.Replicas != idl.Status.UpdatedReplicas {
		logger.Warn("Final state mismatch, skip and retry later")
		return fmt.Errorf("final state mismatch")
	}

	logger.Info("All batches are finished, the deploy is done")
	idl.MarkAsSuccess()

	r.recorder.Event(idl.Unwrap(), corev1.EventTypeNormal, eventReasonDeployed, eventMessageDeployed)

	return nil
}

func (r *DeployFlowReconciler) processNewBatch(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	logger.Info("Start to process new batch")

	startedAt := metav1.Now()

	if err := r.processCloneSet(idl); err != nil {
		logger.WithError(err).Error("failed to process cloneSet")
		return err
	}

	idl.MarkCurrentBatchAsStarted(startedAt)
	logger.Infof("Batch %d is started", idl.CurrentBatchNumber())

	return nil
}

func (r *DeployFlowReconciler) processSmokingBatch(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	logger.Info("Processing smoking batch.")

	if idl.CurrentBatchFailed() {
		logger.Info("Current batch failed, pause the CloneSet")
		if err := r.pauseCloneSet(idl); err != nil {
			return err
		}

		logger.Info("Paused deploy due to batch failure")
		idl.MarkAsPaused()
		return nil
	}

	if idl.CurrentBatchIsContainersReady() {
		return r.processContainersReadyBatch(idl)
	}

	return nil
}

// processSmokedBatch enables new pods by:
// 1. set pod readiness gate to True.
// 2. when all pods are ready, pull in from registry.
// 3. check periodically to make sure all instances are enabled.
func (r *DeployFlowReconciler) processSmokedBatch(idl *internaldeploy.Deploy) error {
	pulledInAt := idl.CurrentBatchPullInAt()
	if pulledInAt.IsZero() {
		return r.prepareToBakeBatch(idl)
	}
	return r.checkBakingGate(idl)
}

func (r *DeployFlowReconciler) processFinishedBatch(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	if idl.AllBatchFinished() {
		idl.MarkAsBatchFinished()
		return nil
	}

	batch := idl.CurrentBatchInfo()
	// if mode is auto and current batch is canary, do not wait batch interval
	if !(idl.Auto() && idl.CurrentBatchIsCanary()) {
		if time.Since(batch.FinishedAt.Time) < time.Duration(idl.BatchIntervalSeconds())*time.Second {
			logger.Info("batch time interval not reached, sleep 3 seconds and try again")
			return terrors.NewTimeIntervalNotReachedError(3 * time.Second)
		}
	}

	logger.Info("Current batch is finished, prepare a new one")

	idl.PrepareNewBatch()

	return nil
}

func (r *DeployFlowReconciler) processEmptyBatch(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	if idl.AllBatchFinished() {
		idl.MarkAsBatchFinished()
		return nil
	}

	logger.Info("no batches is initialized, prepare a new one")

	idl.PrepareNewBatch()

	return nil
}

func (r *DeployFlowReconciler) processContainersReadyBatch(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	logger.Info("All Pods in current batch are ready")

	// only canary pods need to be registered to gateway.
	if idl.CurrentBatchIsCanary() {
		if err := r.registerPods(idl); err != nil {
			logger.WithError(err).Error("failed to register pods")

			// move forward even it is failed
			//return err
		}
	}

	logger.Info("Marking current batch as smoked")
	idl.MarkCurrentBatchAsSmoked()

	return nil
}

func (r *DeployFlowReconciler) prepareToBakeBatch(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	logger.Info("Preparing to bake the batch")

	if !idl.SkipPullIn() {
		logger.Info("Pulling in new pods")

		if !idl.CurrentBatchIsPodReady() {
			return r.setPodReadinessGates(idl)
		}
		// pull in new pods
		//if err := r.pullIn(idl); err != nil {
		//	logger.WithError(err).Error("failed to pull in new pods")
		//	return err
		//}
	} else {
		logger.Info("skip pulling in")
	}

	idl.SetPulledInAt()

	return nil
}

func (r *DeployFlowReconciler) processBakingBatch(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	logger.Info("Baking is done, remove old pods if any and mark current batch as baked")

	// pull out old pods
	//if err := r.pullOut(idl); err != nil {
	//	logger.WithError(err).Error("failed to pull out old pods")
	//	return err
	//}

	logger.Infof("Batch %d is finished", idl.CurrentBatchNumber())
	idl.MarkCurrentBatchAsFinished()

	return nil
}

func (r *DeployFlowReconciler) checkBakingGate(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	if !idl.SkipPullIn() {
		logger.Info("Checking if all pods are pulled in")

		if time.Since(idl.CurrentBatchPullInAt().Time) > 20*time.Second {
			pods := idl.CurrentBatchPods()
			failedPods := make([]string, 0, len(pods))
			for i := range pods {
				if pods[i].PullInStatus != setting.PodPullInSucceeded {
					pods[i].PullInStatus = setting.PodPullInFailed
					failedPods = append(failedPods, pods[i].Name)
				}
			}

			if len(failedPods) > 0 {
				logger.Infof("Timeout waiting for all pods to be enabled in nacos, failed pods are %s", strings.Join(failedPods, ", "))

				batch := idl.CurrentBatchInfo()
				batch.Pods = pods
				idl.SetCondition(*batch)
			}

			logger.Info("Marking current batch as baking")
			idl.MarkCurrentBatchAsBaking()

			return nil
		}

		enabledPods, ok := r.isAllInstancesUp(idl)
		if len(enabledPods) > 0 {
			ep := sets.NewString(enabledPods...)
			pods := idl.CurrentBatchPods()
			for i := range pods {
				if ep.Has(pods[i].Name) {
					pods[i].PullInStatus = setting.PodPullInSucceeded
				}
			}

			batch := idl.CurrentBatchInfo()
			batch.Pods = pods
			idl.SetCondition(*batch)
		}

		if !ok {
			logger.Info("Not all instances are pulled in, checking again")
			return terrors.NewInstanceNotUpError(500 * time.Millisecond)
		}
	}

	logger.Info("Marking current batch as baking")
	idl.MarkCurrentBatchAsBaking()

	return nil
}

func (r *DeployFlowReconciler) setPodReadinessGates(idl *internaldeploy.Deploy) error {
	logger := r.logger.WithField("deploy", idl)

	for _, p := range idl.CurrentBatchPods() {
		logger.Infof("Setting readiness gate for pod %s as True", p.Name)
		err := base.SetPodReadinessGate(idl.Namespace, p.Name, r.Client)
		if err != nil {
			logger.WithError(err).Errorf("Failed to update pod %s", p.Name)
			return err
		}
	}

	return nil
}

func (r *DeployFlowReconciler) isAllInstancesUp(idl *internaldeploy.Deploy) ([]string, bool) {
	logger := r.logger.WithField("deploy", idl)
	logger.Info("Checking instances status")

	var wg sync.WaitGroup
	pods := idl.CurrentBatchPods()
	var finishedPods int32
	mu := &sync.Mutex{}
	enabledPods := make([]string, 0, len(pods))

	for i := range pods {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			ip := pods[i].IP
			phase := pods[i].Phase
			podLogger := logger.WithField("ip", ip).WithField("name", pods[i].Name)

			if pods[i].PullInStatus == setting.PodPullInSucceeded || phase == setting.PodFailed {
				atomic.AddInt32(&finishedPods, 1)
				return
			}

			if phase != setting.PodReady {
				podLogger.Warnf("Exception: pod is in an unexpected phase %s", pods[i].Phase)
				return
			}

			// TODO replace with readinessGate
			//if r.nacosClient.IsInstanceEnabled(idl.Spec.AppName, ip) {
			podLogger.Info("All pods in current batch pull in nacos success")

			mu.Lock()
			enabledPods = append(enabledPods, pods[i].Name)
			mu.Unlock()

			atomic.AddInt32(&finishedPods, 1)
			return
			//}
		}(i)
	}

	wg.Wait()

	return enabledPods, finishedPods == int32(len(pods))
}

func (r *DeployFlowReconciler) registerPods(idl *internaldeploy.Deploy) error {
	logger := log.WithField("deploy_name", idl.Name)

	batch := idl.CurrentBatchInfo()
	if batch == nil {
		return fmt.Errorf("current batch is missing, batches are %v", idl.Status.Conditions)
	}

	var lastErr error
	for i := range batch.Pods {
		logger.Infof("Start to register pod %s.", batch.Pods[i].Name)
		//if err := r.gatewayClient.Add(idl.Spec.AppID, batch.Pods[i].IP, batch.Pods[i].Port); err != nil {
		//	logger.WithError(err).Errorf("failed to resister pod %s", batch.Pods[i].Name)
		//	lastErr = err
		//}
		logger.Infof("Success register pod %s, podIp is %s", batch.Pods[i].Name, batch.Pods[i].IP)
	}

	return lastErr
}

func (r *DeployFlowReconciler) postDeploy(idl *internaldeploy.Deploy) {
	logger := r.logger.WithField("deploy", idl)
	logger.Info("Deploy is finished, releasing resources")

	if err := r.removeCloneSetOwnerWithRetry(idl); err != nil {
		logger.WithError(err).Error("Failed to remove CloneSet owner")
	}
}

// DeleteCloneSetWhenActionIsScaleInZero handle zero replicas cloneset
func (r *DeployFlowReconciler) DeleteCloneSetWhenActionIsScaleInZero(dl *tritonappsv1alpha1.DeployFlow) error {

	idl := internaldeploy.FromDeploy(dl)

	if idl.Spec.Action == setting.ScaleIn && idl.Finished() && *idl.Spec.Application.Replicas == 0 {
		// get cloneSet owned by deployflow
		cs, found, err := fetcher.GetCloneSetInCacheByDeploy(dl, r.Client)
		if err != nil || !found {
			r.logger.Errorf("failed to found cloneSet %s, error is %v", idl.Spec.Application.InstanceName, err)
			return fmt.Errorf("failed to found cloneSet %s, error is %v", idl.Spec.Application.InstanceName, err)
		}
		err = r.Client.Delete(context.TODO(), cs)
		if err != nil {
			r.logger.Errorf("failed to delete cloneSet %s", idl.Spec.Application.InstanceName)
			return err
		}
	}
	return nil
}
