package deployflow

import (
	"io"

	terrors "github.com/triton-io/triton/pkg/errors"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	"github.com/triton-io/triton/pkg/log"
	"github.com/triton-io/triton/pkg/services/response"
	"github.com/triton-io/triton/pkg/setting"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	kubeclient "github.com/triton-io/triton/pkg/kube/client"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	tritonappsv1alpha1 "github.com/triton-io/triton/apis/apps/v1alpha1"
	internaldeploy "github.com/triton-io/triton/pkg/kube/types/deploy"
)

func PatchDeploy(c *gin.Context) {
	name := c.Param("name")
	ns := c.Param("namespace")

	dLogger := log.WithFields(logrus.Fields{
		"namespace": ns,
		"name":      name,
	})

	d := getDeployOrDie(ns, name, c)
	if d == nil {
		return
	}
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()
	cr := mgr.GetAPIReader()

	if internaldeploy.FromDeploy(d).Finished() {
		response.ConflictWithMessage("changes on a finished deploy is not allowed", c)
		return
	}

	var r interface{}
	if internaldeploy.RevisionChanged(d.Spec.Action) {
		r = &tritonappsv1alpha1.DeployUpdateStrategy{}
	} else {
		r = &tritonappsv1alpha1.DeployNonUpdateStrategy{}
	}

	if err := c.ShouldBindJSON(r); err != nil {
		response.BadRequestWithMessage(err.Error(), c)
		return
	}

	d, err := patchDeployStrategy(ns, name, d.Spec.Action, cr, cl, r)
	if err != nil {
		if apierrors.IsNotFound(err) {
			response.NotFound(c)
			return
		}
		dLogger.WithError(err).Error("failed to patch deploy")
		response.ServerErrorWithErrorAndMessage(err, "failed to patch deploy", c)
		return
	}

	rep := setKubeDeployReply(d)
	response.OkDetailed(rep, "success", c)
}

func CreateDeploy(c *gin.Context) {
	ns := c.Param("namespace")

	r := &DeployUpdateRequest{}
	err := c.ShouldBindJSON(r)
	if err != nil {
		response.BadRequestWithMessage(err.Error(), c)
		return
	}

	dLogger := log.WithFields(logrus.Fields{
		"namespace":    ns,
		"instanceName": r.ApplicationSpec.InstanceName,
		"appID":        r.ApplicationSpec.AppID,
		"groupID":      r.ApplicationSpec.GroupID,
	})
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	updated, err := CreateUpdateDeploy(ns, r, cl, dLogger)
	if err != nil {
		if terrors.IsConflict(err) {
			response.ConflictWithMessage(err.Error(), c)
		} else {
			response.ServerErrorWithMessage(err.Error(), c)
		}
		return
	}

	rep := setKubeDeployReply(updated)
	response.Created(rep, c)
	dLogger.Info("Finished to create deploy")
}

func CreateScale(c *gin.Context) {
	ns := c.Param("namespace")
	instanceName := c.Param("name")
	action := setting.Scale

	r := &scaleRequest{}
	err := c.ShouldBindJSON(r)
	if err != nil && !errors.Is(err, io.EOF) {
		response.BadRequestWithMessage(err.Error(), c)
		return
	}

	createNonUpdateDeploy(ns, instanceName, action, r.NonUpdateStrategy, r.Replicas, c)
}

func CreateRollback(c *gin.Context) {
	ns := c.Param("namespace")
	instanceName := c.Param("name")

	r := &rollbackRequest{}
	err := c.ShouldBindJSON(r)
	if err != nil && !errors.Is(err, io.EOF) {
		response.BadRequestWithMessage(err.Error(), c)
		return
	}
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	dLogger := log.WithFields(logrus.Fields{
		"namespace":    ns,
		"instanceName": instanceName,
		"deploy":       r.DeployName,
	})

	updated, oldName, err := RollbackDeploy(ns, instanceName, r.DeployName, cl, r.UpdateStrategy, dLogger)
	if err != nil {
		if terrors.IsNotFound(err) {
			response.NotFound(c)
		} else if terrors.IsConflict(err) {
			response.ConflictWithMessage(err.Error(), c)
		} else {
			response.ServerErrorWithMessage(err.Error(), c)
		}
		return
	}

	rep := setRollbackReply(oldName, updated.Name)
	response.Created(rep, c)
	dLogger.Info("Finished to rollback application")
}

func CreateRestart(c *gin.Context) {
	ns := c.Param("namespace")
	instanceName := c.Param("name")
	action := setting.Restart

	r := &tritonappsv1alpha1.DeployNonUpdateStrategy{}
	err := c.ShouldBindJSON(r)
	if err != nil && !errors.Is(err, io.EOF) {
		response.BadRequestWithMessage(err.Error(), c)
		return
	}

	createNonUpdateDeploy(ns, instanceName, action, r, 0, c)
}

func DeleteDeploy(c *gin.Context) {
	name := c.Param("name")
	ns := c.Param("namespace")

	dLogger := log.WithFields(logrus.Fields{
		"namespace": ns,
		"name":      name,
	})
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	err := RemoveDeploy(ns, name, cl, dLogger)
	if err != nil {
		if terrors.IsNotFound(err) {
			response.Deleted(c)
		} else if terrors.IsConflict(err) {
			response.ConflictWithMessage(err.Error(), c)
		} else {
			response.ServerErrorWithMessage(err.Error(), c)
		}
		return
	}

	response.Deleted(c)
}

func GetDeploy(c *gin.Context) {
	name := c.Param("name")
	ns := c.Param("namespace")

	dLogger := log.WithFields(logrus.Fields{
		"namespace": ns,
		"name":      name,
	})

	dLogger.Info("Getting deploy")
	d := getDeployOrDie(ns, name, c)
	if d == nil {
		return
	}

	rep := setKubeDeployReply(d)
	response.OkDetailed(rep, "success", c)
}

func GetDeploys(c *gin.Context) {
	ns := c.Param("namespace")

	f := &filter{}
	if err := c.ShouldBindQuery(f); err != nil {
		response.BadRequestWithMessage(err.Error(), c)
		return
	}
	if f.PageSize == 0 {
		f.PageSize = 10
	}

	dLogger := log.WithFields(logrus.Fields{
		"namespace": ns,
	})

	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	ds, err := fetcher.GetDeploysInCache(fetcher.DeployFilter{Namespace: ns, InstanceName: f.InstanceName, Start: f.Start, PageSize: f.PageSize}, cl)
	if err != nil {
		dLogger.WithError(err).Error("failed to get deploys in cache")
		response.ServerErrorWithErrorAndMessage(err, "failed to get deploys in cache", c)
		return
	}

	rep := make([]*reply, 0, f.PageSize)
	for i := range ds {
		r := setKubeDeployReply(ds[i])
		rep = append(rep, r)
	}

	response.OkDetailed(rep, "success", c)
}

func getDeployOrDie(ns, name string, c *gin.Context) *tritonappsv1alpha1.DeployFlow {
	d, found, err := fetcher.GetDeployInCache(ns, name, kubeclient.NewManager().GetClient())
	if err != nil {
		response.ServerErrorWithErrorAndMessage(err, "failed to get deploy in cache", c)
		return nil
	} else if !found {
		response.NotFound(c)
		return nil
	}

	return d
}

func createNonUpdateDeploy(ns, instanceName, action string, strategy *tritonappsv1alpha1.DeployNonUpdateStrategy, replicas int32, c *gin.Context) {
	dLogger := log.WithFields(logrus.Fields{
		"namespace":    ns,
		"instanceName": instanceName,
	})

	req := &DeployNonUpdateRequest{
		Replicas:          replicas,
		Namespace:         ns,
		InstanceName:      instanceName,
		Action:            action,
		NonUpdateStrategy: strategy,
	}
	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	updated, err := CreateNonUpdateDeploy(req, cl, dLogger)
	if err != nil {
		if terrors.IsNotFound(err) {
			response.NotFound(c)
		} else if terrors.IsConflict(err) {
			response.ConflictWithMessage(err.Error(), c)
		} else {
			response.ServerErrorWithMessage(err.Error(), c)
		}
		return
	}

	var rep interface{}
	switch action {
	case setting.Restart:
		rep = setRestartReply(updated.Name)
	case setting.Scale, setting.ScaleIn, setting.ScaleOut:
		rep = setScaleReply(updated.Name)

	}

	response.Created(rep, c)
}
