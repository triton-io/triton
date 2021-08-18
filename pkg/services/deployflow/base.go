package deployflow

import (
	"context"
	"fmt"
	"time"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/pkg/errors"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	internaldeploy "github.com/triton-io/triton/pkg/kube/types/deploy"
)

type filter struct {
	InstanceName string `form:"instanceName"`
	Action       string `form:"action"`
	Start        int    `form:"start"`
	PageSize     int    `form:"pageSize"`
}

type rollbackRequest struct {
	DeployName string `json:"deployName"`

	UpdateStrategy *tritonappsv1alpha1.DeployUpdateStrategy `json:"updateStrategy,omitempty"`
}

type scaleRequest struct {
	Replicas int32 `json:"replicas"`

	NonUpdateStrategy *tritonappsv1alpha1.DeployNonUpdateStrategy `json:"nonUpdateStrategy,omitempty"`
}

type rollbackResponse struct {
	RollbackTo string `json:"rollbackTo"`
	DeployName string `json:"deployName"`
}

type restartResponse struct {
	DeployName string `json:"deployName"`
}

type scaleResponse struct {
	DeployName string `json:"deployName"`
}

type reply struct {
	Name         string `json:"name"`
	AppID        int    `json:"appID"`
	GroupID      int    `json:"groupID"`
	Namespace    string `json:"namespace"`
	AppName      string `json:"appName"`
	InstanceName string `json:"instanceName"`
	Replicas     int32  `json:"replicas"`
	Action       string `json:"action"`
	Mode         string `json:"mode,omitempty"`

	AvailableReplicas    int32                               `json:"availableReplicas"`
	UpdatedReplicas      int32                               `json:"updatedReplicas"`
	UpdatedReadyReplicas int32                               `json:"updatedReadyReplicas"`
	UpdateRevision       string                              `json:"updateRevision"`
	Conditions           []tritonappsv1alpha1.BatchCondition `json:"conditions"`
	Paused               bool                                `json:"paused"`
	Phase                tritonappsv1alpha1.DeployPhase      `json:"phase"`
	Finished             bool                                `json:"finished"`
	Batches              int                                 `json:"batches"`
	FinishedBatches      int                                 `json:"finishedBatches"`
	FinishedReplicas     int                                 `json:"finishedReplicas"`
	FailedReplicas       int                                 `json:"failedReplicas"`
	StartedAt            metav1.Time                         `json:"startedAt,omitempty"`
	FinishedAt           metav1.Time                         `json:"finishedAt,omitempty"`
	UpdatedAt            metav1.Time                         `json:"updatedAt,omitempty"`
}

func patch(ns, name string, patchBytes []byte, reader client.Reader, cl client.Client) (*tritonappsv1alpha1.DeployFlow, error) {
	err := cl.Patch(context.TODO(), &tritonappsv1alpha1.DeployFlow{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		// difference between MergePatchType and StrategicMergePatchType: https://support.huaweicloud.com/api-cce/cce_02_0086.html
		// CRD does not support StrategicMergePatch
	}, client.RawPatch(types.MergePatchType, patchBytes))
	if err != nil {
		return nil, errors.Wrap(err, "failed to patch deploy")
	}

	updated, err := fetcher.GetDeployFromAPIServer(ns, name, reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get application")
	}

	return updated, nil
}

func create(deploy *tritonappsv1alpha1.DeployFlow, cl client.Client) (*tritonappsv1alpha1.DeployFlow, error) {
	// It is not the origin deploy, Create will populate the latest state and save it to deploy.
	err := cl.Create(context.TODO(), deploy)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create deploy")
	}

	var updated *tritonappsv1alpha1.DeployFlow

	// wait till new deploy object is synced to local cache
	err = wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		var found bool
		updated, found, err = fetcher.GetDeployInCache(deploy.Namespace, deploy.Name, cl)
		if err != nil {
			return false, err
		} else if !found {
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to get deploy")
	}

	return updated, nil
}

func preSteps(cs *kruiseappsv1alpha1.CloneSet, action string, cl client.Client) error {
	if cs == nil {
		return nil
	}

	deploy, found, err := fetcher.GetDeployInCacheOwningCloneSet(cs, cl)
	if err != nil {
		return err
	} else if !found {
		return nil
	}

	idl := internaldeploy.FromDeploy(deploy)
	if !idl.Finished() {
		if !idl.Paused() || !internaldeploy.RevisionChanged(action) {
			return fmt.Errorf("last deploy %s in progress", idl.Name)
		}
	}

	return nil
}

func setKubeDeployReply(deploy *tritonappsv1alpha1.DeployFlow) *reply {
	c := deploy.Status.Conditions
	if len(c) == 0 {
		c = make([]tritonappsv1alpha1.BatchCondition, 0)
	}

	return SetDeploy(deploy)
}

func setRollbackReply(rollbackTo, deploy string) *rollbackResponse {
	return &rollbackResponse{
		RollbackTo: rollbackTo,
		DeployName: deploy,
	}
}

func setRestartReply(deploy string) *restartResponse {
	return &restartResponse{
		DeployName: deploy,
	}
}

func setScaleReply(deploy string) *scaleResponse {
	return &scaleResponse{
		DeployName: deploy,
	}
}
