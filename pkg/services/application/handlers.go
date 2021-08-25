package application

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/triton-io/triton/pkg/kube/fetcher"
	"github.com/triton-io/triton/pkg/log"
	"github.com/triton-io/triton/pkg/services/response"
	"github.com/triton-io/triton/pkg/setting"

	terrors "github.com/triton-io/triton/pkg/errors"
	kubeclient "github.com/triton-io/triton/pkg/kube/client"
)

func GetApplication(c *gin.Context) {
	name := c.Param("name")
	ns := c.Param("namespace")

	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	cs, found, err := fetcher.GetCloneSetInCache(ns, name, cl)
	if err != nil {
		response.ServerErrorWithErrorAndMessage(err, "failed to get application", c)
		return
	} else if !found {
		response.NotFound(c)
		return
	}

	rep := setApplicationReply(cs)
	response.OkDetailed(rep, "success", c)
}

func GetApplications(c *gin.Context) {
	ns := c.Param("namespace")

	aLogger := log.WithField("namespace", ns)

	mgr := kubeclient.NewManager()
	cl := mgr.GetClient()

	css, err := fetcher.GetCloneSetsInCache(ns, cl)
	if err != nil {
		aLogger.WithError(err).Error("failed to get applications")
		response.ServerErrorWithErrorAndMessage(err, "failed to get applications", c)
		return
	}

	rep := make([]*reply, 0, len(css))
	for _, cs := range css {
		r := setApplicationReply(cs)
		rep = append(rep, r)
	}
	response.OkDetailed(rep, "success", c)
}

func DeleteApplication(c *gin.Context) {
	name := c.Param("name")
	ns := c.Param("namespace")

	aLogger := log.WithFields(logrus.Fields{
		"action":    setting.Delete,
		"name":      name,
		"namespace": ns,
	})

	err := RemoveApplication(ns, name, aLogger)
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
