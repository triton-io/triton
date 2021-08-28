package application

import (
	"github.com/gin-gonic/gin"
)

type Router struct{}

func (*Router) SetupRouters(router *gin.RouterGroup) *gin.RouterGroup {
	router.GET("/namespaces/:namespace/instances", GetApplications)
	router.GET("/namespaces/:namespace/instances/:name", GetApplication)
	router.DELETE("/namespaces/:namespace/instances/:name", DeleteApplication)

	return router
}
