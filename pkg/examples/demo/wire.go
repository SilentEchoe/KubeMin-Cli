//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"KubeMin-Cli/pkg/examples/demo/handler"
	"KubeMin-Cli/pkg/examples/demo/ioc"
	"KubeMin-Cli/pkg/examples/demo/service"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

func InitializeApp() *gin.Engine {
	wire.Build(handler.NewApplicationHandler, service.NewApplicationService, ioc.NewGinEngineAndRegisterRoute)
	return &gin.Engine{}
}
