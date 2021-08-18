package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/triton-io/triton/pkg/services/deployflow"
)

type Register interface {
	SetupRouters(router *gin.RouterGroup) *gin.RouterGroup
}

func (s *server) InitRestRoutes(router *gin.RouterGroup) gin.IRoutes {

	for _, r := range []Register{
		new(deployflow.Router),
	} {
		router = r.SetupRouters(router)
	}

	return router
}
