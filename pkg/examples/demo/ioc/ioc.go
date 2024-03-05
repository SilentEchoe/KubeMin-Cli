package ioc

import (
	"KubeMin-Cli/pkg/examples/demo/handler"
	"github.com/gin-gonic/gin"
)

func NewGinEngineAndRegisterRoute(applicationHandler *handler.ApplicationHandler) *gin.Engine {
	engine := gin.Default()
	applicationHandler.RegisterRoutes(engine)
	return engine
}
