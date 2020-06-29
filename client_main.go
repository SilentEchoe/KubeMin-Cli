package main

import (
	"github.com/gin-gonic/gin"
	"github.com/micro/go-plugins/registry/consul"
	"github.com/micro/micro/registry"
)

func main() {
	consulReg := consul.NewRegistry(
		registry.Addrs("http://127.0.0.1:8500"),
		)

	ginRouter := gin.Default()

}

