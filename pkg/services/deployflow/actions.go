package deployflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	terrors "github.com/triton-io/triton/pkg/errors"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	internalcloneset "github.com/triton-io/triton/pkg/kube/types/cloneset"
	internaldeploy "github.com/triton-io/triton/pkg/kube/types/deploy"
	"github.com/triton-io/triton/pkg/log"
	"github.com/triton-io/triton/pkg/setting"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeployNonUpdateRequest struct {
	Replicas          int32
	Namespace         string
	InstanceName      string
	Action            string
	NonUpdateStrategy *tritonappsv1alpha1.DeployNonUpdateStrategy
}

type DeployUpdateRequest struct {
	ApplicationSpec *tritonappsv1alpha1.ApplicationSpec      `json:"applicationSpec"`
	UpdateStrategy  *tritonappsv1alpha1.DeployUpdateStrategy `json:"updateStrategy,omitempty"`
}

func patchDeployStrategy(ns, name, action string, reader client.Reader, cl client.Client, r interface{}) (*tritonappsv1alpha1.DeployFlow, error) {
	strategy, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	return PatchDeployStrategy(ns, name, action, reader, cl, strategy)
}

func PatchDeployStrategy(ns, name, action string, reader client.Reader, cl client.Client, strategyBytes []byte) (*tritonappsv1alpha1.DeployFlow, error) {
	var patchBytes []byte
	if internaldeploy.RevisionChanged(action) {
		patchBytes = []byte(fmt.Sprintf(`{"spec":{"updateStrategy":%s}}`, strategyBytes))
	} else {
		patchBytes = []byte(fmt.Sprintf(`{"spec":{"nonUpdateStrategy":%s}}`, strategyBytes))
	}
	log.WithField("deploy", name).Debugf("patchBytes is %s", patchBytes)

	return patch(ns, name, patchBytes, reader, cl)
}

func PatchDeployStatus(ns, name string, reader client.Reader, cl client.Client, strategyBytes []byte) (*tritonappsv1alpha1.DeployFlow, error) {
	log.WithField("deploy", name).Debugf("patchBytes is %s", strategyBytes)

	return patch(ns, name, strategyBytes, reader, cl)
}

func CreateNonUpdateDeploy(r *DeployNonUpdateRequest, cl client.Client, logger *logrus.Entry) (*tritonappsv1alpha1.DeployFlow, error) {
	action := r.Action
	logger.Infof("Start to %s application", action)

	cs, found, err := fetcher.GetCloneSetInCache(r.Namespace, r.InstanceName, cl)
	if err != nil {
		logger.WithError(err).Error("failed to fetch cloneSet")
		return nil, err
	} else if !found {
		return nil, terrors.NewNotFound("cloneSet not found")
	}

	if err := preSteps(cs, action, cl); err != nil {
		logger.WithError(err).Error("pre steps failed")
		return nil, terrors.NewConflict("pre steps failed", err)
	}

	// it is a scale action if replicas > 0
	replicas := r.Replicas
	if replicas >= 0 && action == setting.Scale {
		if replicas >= *cs.Spec.Replicas {
			action = setting.ScaleOut
		} else {
			action = setting.ScaleIn
		}
	} else {
		replicas = *cs.Spec.Replicas
	}
	ics := internalcloneset.FromCloneSet(cs)
	g := generator{
		appID:             ics.GetAppID(),
		groupID:           ics.GetGroupID(),
		replicas:          replicas,
		namespace:         r.Namespace,
		appName:           ics.GetAppName(),
		instanceName:      r.InstanceName,
		action:            action,
		nonUpdateStrategy: r.NonUpdateStrategy,
	}
	deploy := g.generate()

	updated, err := create(deploy, cl)
	if err != nil {
		logger.WithError(err).Error("failed to create deploy")
		return nil, err
	}
	logger.Infof("Finished to %s application", action)

	return updated, nil
}

func CreateUpdateDeploy(ns string, r *DeployUpdateRequest, cl client.Client, logger *logrus.Entry) (*tritonappsv1alpha1.DeployFlow, error) {
	logger.Info("Start to create new deploy")

	cs, found, err := fetcher.GetCloneSetInCache(ns, r.ApplicationSpec.InstanceName, cl)
	if err != nil {
		logger.WithError(err).Error("failed to get application")
		return nil, err
	}

	action := setting.Create
	if found && cs != nil {
		action = setting.Update
	}

	if err := preSteps(cs, action, cl); err != nil {
		logger.WithError(err).Error("pre steps failed")
		return nil, terrors.NewConflict("pre steps failed", err)
	}

	var replicas int32 = 1
	if r.ApplicationSpec != nil && r.ApplicationSpec.Replicas != nil {
		replicas = *r.ApplicationSpec.Replicas
	} else if cs != nil && cs.Spec.Replicas != nil {
		replicas = *cs.Spec.Replicas
	}

	g := generator{
		appID:           r.ApplicationSpec.AppID,
		groupID:         r.ApplicationSpec.GroupID,
		replicas:        replicas,
		namespace:       ns,
		appName:         r.ApplicationSpec.AppName,
		instanceName:    r.ApplicationSpec.InstanceName,
		action:          action,
		applicationSpec: r.ApplicationSpec,
		updateStrategy:  r.UpdateStrategy,
	}
	deploy := g.generate()

	updated, err := create(deploy, cl)
	if err != nil {
		logger.WithError(err).Error("failed to create deploy")
		return nil, err
	}
	logger.Info("Finished to create deploy")

	return updated, nil
}

func RollbackDeploy(ns, instanceName, deployName string, cl client.Client, strategy *tritonappsv1alpha1.DeployUpdateStrategy, logger *logrus.Entry) (*tritonappsv1alpha1.DeployFlow, string, error) {
	action := setting.Rollback

	cs, _, _ := fetcher.GetCloneSetInCache(ns, instanceName, cl)

	if err := preSteps(cs, action, cl); err != nil {
		logger.WithError(err).Error("pre steps failed")
		return nil, "", terrors.NewConflict("pre steps failed", err)
	}

	var deploy *tritonappsv1alpha1.DeployFlow = nil
	var err error
	if deployName == "" {
		// TODO: get latest revision from k8s cloneSet ControllerRevision objects
		// k8s.io/kubectl/pkg/polymorphichelpers/rollback.go
		// deploy, err = getLastSuccessfulDeployByRevisionInCache(ns, "revision", cl)
	} else {
		deploy, _, err = fetcher.GetDeployInCache(ns, deployName, cl)
	}

	if err != nil || deploy == nil {
		logger.WithError(err).Error("failed to get deploy")
		return nil, "", fmt.Errorf("failed to get deploy: %w", err)
	}

	logger.Infof("Start to rollback to deploy %s", deploy.Name)

	oldName := deploy.Name

	// modify the old deploy
	deploy.SetName("")
	deploy.Spec.Action = action
	deploy.Spec.UpdateStrategy = strategy
	deploy.SetResourceVersion("")

	// do not change replicas
	if cs != nil && cs.Spec.Replicas != nil {
		*deploy.Spec.Application.Replicas = *cs.Spec.Replicas
	}

	updated, err := create(deploy, cl)
	if err != nil {
		logger.WithError(err).Error("failed to create deploy")
		return nil, "", fmt.Errorf("failed to create deploy: %w", err)
	}

	logger.Info("Finished to rollback application")

	return updated, oldName, nil
}

func RemoveDeploy(ns, name string, cl client.Client, logger *logrus.Entry) error {

	d, found, err := fetcher.GetDeployInCache(ns, name, cl)
	if err != nil {
		logger.WithError(err).Error("failed to get deploy")
		return err
	} else if !found {
		return terrors.NewNotFound(fmt.Sprintf("deploy %s not found", name))
	}

	if !internaldeploy.FromDeploy(d).Finished() {
		return terrors.NewConflict("deleting a not finished deploy is not allowed", nil)
	}

	logger.Info("Start to delete deploy")
	if err := cl.Delete(context.TODO(), &tritonappsv1alpha1.DeployFlow{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.WithError(err).Error("failed to delete deploy")
			return err
		}
	}

	// wait till deploy is deleted from local cache
	err = wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		_, found, err := fetcher.GetDeployInCache(ns, name, cl)
		if err != nil {
			return false, err
		} else if found {
			return false, nil
		}
		return true, nil
	})

	logger.Info("Finished to delete deploy")

	return nil
}

func SetDeploy(deploy *tritonappsv1alpha1.DeployFlow) *reply {

	c := deploy.Status.Conditions
	if len(c) == 0 {
		c = make([]tritonappsv1alpha1.BatchCondition, 0)
	}

	return &reply{
		Name:         deploy.Name,
		AppID:        deploy.Spec.Application.AppID,
		GroupID:      deploy.Spec.Application.GroupID,
		Namespace:    deploy.Namespace,
		AppName:      deploy.Spec.Application.AppName,
		InstanceName: deploy.Spec.Application.InstanceName,
		Replicas:     *deploy.Spec.Application.Replicas,
		Action:       deploy.Spec.Action,
		Mode:         string(internaldeploy.FromDeploy(deploy).Mode()),

		AvailableReplicas:    deploy.Status.AvailableReplicas,
		UpdatedReplicas:      deploy.Status.UpdatedReplicas,
		UpdatedReadyReplicas: deploy.Status.UpdatedReadyReplicas,
		UpdateRevision:       internaldeploy.FromDeploy(deploy).GetUpdateRevision(),
		Conditions:           c,
		Paused:               deploy.Status.Paused,
		Phase:                deploy.Status.Phase,
		Finished:             deploy.Status.Finished,
		Batches:              deploy.Status.Batches,
		FinishedBatches:      deploy.Status.FinishedBatches,
		FinishedReplicas:     deploy.Status.FinishedReplicas,
		FailedReplicas:       deploy.Status.FailedReplicas,
		StartedAt:            deploy.Status.StartedAt,
		FinishedAt:           deploy.Status.FinishedAt,
		UpdatedAt:            deploy.Status.UpdatedAt,
	}
}
