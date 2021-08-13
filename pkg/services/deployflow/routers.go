package deployflow

import (
	"github.com/gin-gonic/gin"
)

type Router struct{}

func (*Router) SetupRouters(router *gin.RouterGroup) *gin.RouterGroup {
	router.GET("/namespaces/:namespace/deployflows", GetDeploys)
	router.GET("/namespaces/:namespace/deployflows/:name", GetDeploy)
	router.POST("/namespaces/:namespace/deployflows", CreateDeploy)
	router.PATCH("/namespaces/:namespace/deployflows/:name", PatchDeploy)
	router.DELETE("/namespaces/:namespace/deployflows/:name", DeleteDeploy)
	router.POST("/namespaces/:namespace/instances/:name/rollbacks", CreateRollback)
	router.POST("/namespaces/:namespace/instances/:name/restarts", CreateRestart)
	router.POST("/namespaces/:namespace/instances/:name/scales", CreateScale)

	return router
}
