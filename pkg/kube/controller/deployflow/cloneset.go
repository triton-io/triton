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
	"fmt"

	"github.com/sirupsen/logrus"
	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	"github.com/triton-io/triton/pkg/setting"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	internaldeploy "github.com/triton-io/triton/pkg/kube/types/deploy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *DeployFlowReconciler) removeCloneSetOwner(idl *internaldeploy.Deploy) error {
	cs := &kruiseappsv1alpha1.CloneSet{}
	err := r.reader.Get(context.TODO(), types.NamespacedName{Namespace: idl.Namespace, Name: idl.Spec.Application.InstanceName}, cs)
	if err != nil {
		return err
	}

	if !metav1.IsControlledBy(cs, idl.Unwrap()) {
		return nil
	}

	// TODO: does the cloneSet have other owners?
	cs.SetOwnerReferences(nil)

	return r.Update(context.TODO(), cs)
}

func (r *DeployFlowReconciler) setCloneSetOwner(idl *internaldeploy.Deploy) error {
	cs := &kruiseappsv1alpha1.CloneSet{}
	err := r.reader.Get(context.TODO(), types.NamespacedName{Namespace: idl.Namespace, Name: idl.Spec.Application.InstanceName}, cs)
	if err != nil {
		return err
	}

	err = controllerutil.SetControllerReference(idl.Unwrap(), cs, r.Scheme)
	if err != nil {
		return err
	}

	return r.Update(context.TODO(), cs)
}

func (r *DeployFlowReconciler) removeCloneSetOwnerWithRetry(idl *internaldeploy.Deploy) error {
	if err := r.removeCloneSetOwner(idl); err != nil {
		if apierrors.IsConflict(err) {
			return r.removeCloneSetOwner(idl)
		}
		return err
	}

	return nil
}

func (r *DeployFlowReconciler) takeOwnershipOfCloneSet(idl *internaldeploy.Deploy) error {
	cs := &kruiseappsv1alpha1.CloneSet{}
	err := r.reader.Get(context.TODO(), types.NamespacedName{Namespace: idl.Namespace, Name: idl.Spec.Application.InstanceName}, cs)
	if err != nil {
		return err
	}

	//cs.SetOwnerReferences(nil)

	err = controllerutil.SetControllerReference(idl.Unwrap(), cs, r.Scheme)
	if err != nil {
		return err
	}

	// resume the CloneSet when taking ownership
	cs.Spec.UpdateStrategy.Paused = false

	return r.Update(context.TODO(), cs)
}

func (r *DeployFlowReconciler) pauseCloneSet(idl *internaldeploy.Deploy) error {
	return r.pauseOrResumeCloneSet(idl, true)
}

func (r *DeployFlowReconciler) resumeCloneSet(idl *internaldeploy.Deploy) error {
	return r.pauseOrResumeCloneSet(idl, false)
}

func (r *DeployFlowReconciler) pauseOrResumeCloneSet(idl *internaldeploy.Deploy, paused bool) error {
	action := "pause"
	if !paused {
		action = "resume"
	}

	klog.V(4).Infof("Start to %s cloneSet.", action)

	patchBytes := []byte(fmt.Sprintf(`{"spec":{"updateStrategy":{"paused":%t}}}`, paused))

	return PatchCloneSet(idl.Unwrap(), patchBytes, r.Client)
}

// processCloneSet updates the cloneSet to make progress.
func (r *DeployFlowReconciler) processCloneSet(idl *internaldeploy.Deploy) error {
	klog.V(4).Info("Start to process a cloneSet.")

	patchBytes := r.getPatchBytes(idl)
	if len(patchBytes) == 0 {
		return nil
	}

	klog.Infof("Update cloneSet, patchBytes %s", string(patchBytes))
	return PatchCloneSet(idl.Unwrap(), patchBytes, r.Client)
}

// getPatchBytes returns the patch bytes for update
// 1. if it is a Create, we should increase the replicas
// 2. if it is a Update in batch pending stage, we should increase the replicas
// 3. if it is a Update in batch baking stage, we should decrease the replicas and partition
// 4. if it is a Update in the first batch pending stage, and there are already several updated
//    replicas (it may happen in a rollback), we should adjust the replicas and partition accordingly。
func (r *DeployFlowReconciler) getPatchBytes(idl *internaldeploy.Deploy) []byte {
	logger := r.logger.WithField("deploy", idl)
	action := idl.Spec.Action

	switch action {
	case setting.Create:
		replicas := idl.Status.FinishedReplicas + idl.CurrentBatchSize()
		if replicas > int(*idl.Spec.Application.Replicas) {
			// should not happened here, something must be wrong.
			klog.Errorf("invalid replicas!!!")
			logger.WithFields(logrus.Fields{
				"replicas":         replicas,
				"finishedReplicas": idl.Status.FinishedReplicas,
				"currentBatchSize": idl.CurrentBatchSize(),
			}).Errorf("invalid replicas!!!")
		}

		return []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	case setting.Update, setting.Rollback:
		switch idl.CurrentBatchPhase() {
		case tritonappsv1alpha1.BatchPending:
			replicas := int(*idl.Spec.Application.Replicas) + idl.CurrentBatchSize()

			if idl.CurrentBatchNumber() == 1 && idl.Status.UpdatedReplicas > 0 {
				if idl.Status.UpdatedReplicas >= *idl.Spec.Application.Replicas {
					return nil
				}
				partition := int(*idl.Spec.Application.Replicas) - int(idl.Status.UpdatedReplicas)
				return []byte(fmt.Sprintf(`{"spec":{"replicas":%d,"updateStrategy":{"partition":%d}}}`, replicas, partition))
			}

			return []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
		case tritonappsv1alpha1.BatchBaking:
			replicas := int(*idl.Spec.Application.Replicas)
			partition := replicas - int(idl.Status.UpdatedReplicas)
			if partition < 0 {
				// should not happened here, something must be wrong.
				logger.WithFields(logrus.Fields{
					"replicas":         replicas,
					"finishedReplicas": idl.Status.FinishedReplicas,
					"currentBatchSize": idl.CurrentBatchSize(),
				}).Errorf("invalid partition!!!")
			}

			return []byte(fmt.Sprintf(`{"spec":{"replicas":%d,"updateStrategy":{"partition":%d}}}`, replicas, partition))
		}
	case setting.Restart:
		switch idl.CurrentBatchPhase() {
		case tritonappsv1alpha1.BatchPending:
			replicas := int(*idl.Spec.Application.Replicas) + idl.CurrentBatchSize()
			return []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
		case tritonappsv1alpha1.BatchBaking:
			replicas := int(*idl.Spec.Application.Replicas)
			return []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
		}
	case setting.ScaleOut:
		switch idl.CurrentBatchPhase() {
		case tritonappsv1alpha1.BatchPending:
			replicas := int(idl.Status.Replicas) + idl.CurrentBatchSize()
			return []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
		}
	case setting.ScaleIn:
		switch idl.CurrentBatchPhase() {
		case tritonappsv1alpha1.BatchBaking:
			replicas := int(idl.Status.Replicas) - idl.CurrentBatchSize()
			return []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
		}
	}

	return nil
}

func (r *DeployFlowReconciler) createCloneSet(idl *internaldeploy.Deploy) error {
	cs, err := generateCloneSet(idl, r.Scheme)
	if err != nil {
		return err
	}

	r.logger.WithField("deploy", idl).Infof("Creating CloneSet %s", cs.Name)
	return r.Create(context.TODO(), cs)
}

func (r *DeployFlowReconciler) updateCloneSet(idl *internaldeploy.Deploy) error {
	ctx := context.Background()

	cs, err := generateCloneSet(idl, r.Scheme)
	if err != nil {
		return err
	}

	tmp := &kruiseappsv1alpha1.CloneSet{}
	if err := r.reader.Get(ctx, types.NamespacedName{Namespace: idl.Namespace, Name: idl.Spec.Application.InstanceName}, tmp); err != nil {
		return err
	}

	cs.SetResourceVersion(tmp.GetResourceVersion())

	// always hold the update, let Deploy controller to make progress
	cs.Spec.Replicas = idl.Spec.Application.Replicas
	p := intstr.FromInt(int(*idl.Spec.Application.Replicas))
	cs.Spec.UpdateStrategy.Partition = &p

	r.logger.WithField("deploy", idl).Infof("Updating CloneSet %s", cs.Name)
	return r.Update(ctx, cs)
}

func generateCloneSet(idl *internaldeploy.Deploy, scheme *runtime.Scheme) (*kruiseappsv1alpha1.CloneSet, error) {

	cs := generate(idl)

	_ = controllerutil.SetControllerReference(idl.Unwrap(), cs, scheme)
	return cs, nil
}

func PatchCloneSet(deploy *tritonappsv1alpha1.DeployFlow, patchBytes []byte, cl client.Client) error {
	cs, _, _ := fetcher.GetCloneSetInCacheOwnedByDeploy(deploy, cl)
	if cs == nil {
		return nil
	}

	err := cl.Patch(context.TODO(), &kruiseappsv1alpha1.CloneSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: deploy.Namespace,
			Name:      deploy.Spec.Application.InstanceName,
		},
	}, client.RawPatch(types.MergePatchType, patchBytes))
	if err != nil {
		return err
	}

	return nil
}
