package main

import (
	"github.com/gin-gonic/gin"

	"github.com/micro/go-micro/web"

	"github.com/micro/go-micro/registry"

	"github.com/micro/go-plugins/registry/consul"
	_  "github.com/micro/go-plugins/registry/consul"
)

func main()  {
	consulReg := consul.NewRegistry(
		registry.Addrs("182.168.29.135:8500"),
		)

	ginRouter := gin.Default()
	ginRouter.Handle("GET","/user", func(context *gin.Context) {
		context.String(200,"user api")
	})

	server := web.NewService(
		web.Name("prodservice"),
		web.Address(":8001"),
		web.Handler(ginRouter),
		web.Registry(consulReg),
		)
	server.Run()

}
