package cloneset

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	internaldeploy "github.com/triton-io/triton/pkg/kube/types/deploy"
	internalpod "github.com/triton-io/triton/pkg/kube/types/pod"
	"github.com/triton-io/triton/pkg/log"
	"github.com/triton-io/triton/pkg/setting"
)

// CloneSetReconciler reconciles a CloneSet object
type CloneSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	logger *logrus.Entry
}

// Add creates a new CloneSet Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newCloneSetReconciler(mgr))
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {

	err := ctrl.NewControllerManagedBy(mgr).
		For(&kruiseappsv1alpha1.CloneSet{}).
		Owns(&corev1.Pod{}).
		Complete(r)

	if err != nil {
		return err
	}

	log.Info("CloneSet Controller created")

	return nil
}

func newCloneSetReconciler(mgr ctrl.Manager) *CloneSetReconciler {
	logger := log.WithField("controller", "CloneSet")

	return &CloneSetReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		logger: logger,
	}
}

// +kubebuilder:rbac:groups=apps.kruise.io,resources=clonesets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.kruise.io,resources=clonesets/status,verbs=get

func (r *CloneSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.logger.WithField("cloneSet", req.NamespacedName)

	cs := &kruiseappsv1alpha1.CloneSet{}
	if err := r.Get(ctx, req.NamespacedName, cs); err != nil {
		logger.WithError(err).Error("unable to fetch CloneSet")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	deploy, found, err := fetcher.GetDeployInCacheOwningCloneSet(cs, r.Client)
	if err != nil || !found {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	idl := internaldeploy.FromDeploy(deploy)

	err = r.syncPodStatusInPreviousBatches(cs, idl.Unwrap())
	if err != nil {
		r.logger.WithError(err).Errorf("sync batch pod status failed")
		return ctrl.Result{}, nil
	}
	r.logger.Infof("sync batch pod status succeed")

	if idl.Finished() || idl.Paused() || idl.NotStarted() {
		// logger.Info("deploy is finished, paused or not started, skip reconciling")
		return ctrl.Result{}, nil
	}

	// logger.Info("Start to reconcile")

	if err := r.populatePods(ctx, cs, deploy); err != nil {
		logger.WithError(err).Error("failed to populate pods")
		return ctrl.Result{}, err
	}

	// logger.Info("Finished to reconcile")
	return ctrl.Result{}, nil
}

func (r *CloneSetReconciler) populatePods(ctx context.Context, cs *kruiseappsv1alpha1.CloneSet, deploy *tritonappsv1alpha1.DeployFlow) error {
	idl := internaldeploy.FromDeploy(deploy)

	currentBatchInfo := idl.CurrentBatchInfo()
	if currentBatchInfo == nil {
		return nil
	}
	// populate pods in smoking and smoked (before pull in) stage
	if !idl.CurrentBatchSmoking() && !idl.CurrentBatchSmoked() {
		return nil
	}

	pods := &corev1.PodList{}
	updatedPodsLabel := map[string]string{
		appsv1.ControllerRevisionHashLabelKey: cs.Status.UpdateRevision,
	}
	if err := r.List(ctx, pods, client.InNamespace(cs.Namespace), client.MatchingLabels(updatedPodsLabel)); err != nil {
		// do not retry, wait for next reconcile
		return nil
	}

	populatedPods := sets.NewString(deploy.Status.Pods...)
	batchPods := sets.NewString()
	failedPods := sets.NewString()
	for i := range currentBatchInfo.Pods {
		batchPods.Insert(currentBatchInfo.Pods[i].Name)
	}

	ps := make([]tritonappsv1alpha1.PodInfo, 0, currentBatchInfo.BatchSize)
	for _, p := range pods.Items {
		// if a pod is in populatedPods but not in batchPods, or it is created before batch is started, it is a pod in old batches.
		if populatedPods.Has(p.Name) && !batchPods.Has(p.Name) || p.CreationTimestamp.Before(&currentBatchInfo.StartedAt) {
			continue
		}

		r.logger.Infof("populate pod %s", p.Name)

		batchPods.Insert(p.Name)
		populatedPods.Insert(p.Name)

		ip := internalpod.FromPod(&p)
		ps = append(ps, tritonappsv1alpha1.PodInfo{
			Name:  p.Name,
			IP:    ip.GetPodIP(),
			Port:  ip.GetAppPort(),
			Phase: string(ip.GetPhase()),
		})

		if ip.Failed() {
			failedPods.Insert(p.Name)
		}
	}

	sort.Slice(ps, func(i, j int) bool {
		return ps[i].Name < ps[j].Name
	})
	currentBatchInfo.FailedReplicas = failedPods.Len()
	currentBatchInfo.Pods = ps

	idl.SetPodsStatus(populatedPods.List())
	idl.SetCondition(*currentBatchInfo)

	return r.updateDeployStatus(idl)
}

func (r *CloneSetReconciler) syncPodStatusInPreviousBatches(cs *kruiseappsv1alpha1.CloneSet, deploy *tritonappsv1alpha1.DeployFlow) error {
	pods := &corev1.PodList{}
	updatedPodsLabel := map[string]string{
		appsv1.ControllerRevisionHashLabelKey: cs.Status.UpdateRevision,
	}
	if err := r.List(context.TODO(), pods, client.InNamespace(cs.Namespace), client.MatchingLabels(updatedPodsLabel)); err != nil {
		// do not retry, wait for next reconcile
		return nil
	}

	podMap := sync.Map{} // for concurrnency handle
	for _, p := range pods.Items {
		tmp := p
		r.logger.Infof("the map key is %s", tmp.Name)
		podMap.Store(p.Name, internalpod.FromPod(&tmp))
	}

	idl := internaldeploy.FromDeploy(deploy)
	var changed bool
	for i := idl.CurrentBatchNumber() - 1; i > 0; i-- {
		batch := idl.Status.Conditions[i] // i-1
		var failed int32
		for j := range batch.Pods {
			r.logger.Infof("the batch pod j name is %s, phase is %s", batch.Pods[j].Name, batch.Pods[j].Phase)
			if p1, ok := podMap.Load(batch.Pods[j].Name); ok {
				// pod is crashed/restarted
				p := p1.(*internalpod.Pod)
				r.logger.Infof("the p is %s, phase is %s", p.Name, p.GetPhase())
				if batch.Pods[j].Phase == setting.PodReady && string(p.GetPhase()) != setting.PodReady {
					batch.Pods[j].Phase = string(p.GetPhase())
					r.logger.Infof("the after batch pod name is  %s, phase is %s", batch.Pods[j].Name, batch.Pods[j].Phase)
					changed = true
					atomic.AddInt32(&failed, 1)
				}

				// pod from failed become Ready or ContainerReady, continue the deployflow
				if batch.Pods[j].Phase == setting.PodFailed && string(p.GetPhase()) != setting.PodFailed {
					batch.Pods[j].Phase = string(p.GetPhase())
					r.logger.Infof("the after batch pod name is  %s, phase is %s", batch.Pods[j].Name, batch.Pods[j].Phase)
					changed = true
					atomic.AddInt32(&failed, -1)
					idl.Status.Paused = false
				}
			} else {
				if deploy.Spec.Action != setting.Restart {
					// pod is gone, maybe caused by a node crash
					batch.Pods[j].Phase = setting.PodOutdated
					changed = true
					atomic.AddInt32(&failed, 1)
				}
			}
		}
		if failed != 0 {
			batchFailedReplicas := int32(batch.FailedReplicas)
			batch.FailedReplicas = int(atomic.AddInt32(&batchFailedReplicas, failed))
			r.logger.Infof("after calculation failed replicas is %d", batch.FailedReplicas)
			idl.SetCondition(batch)
		}
	}

	if changed {
		return r.updateDeployStatus(idl)
	}

	return nil
}

func (r *CloneSetReconciler) updateDeployStatus(idl *internaldeploy.Deploy) error {
	return r.Status().Update(context.TODO(), idl.Unwrap())
}
