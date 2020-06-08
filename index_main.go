package main

import (
	"github.com/gin-gonic/gin"

	"github.com/micro/go-micro/web"

	_ "github.com/micro/go-plugins/registry/consul"
)

func main()  {


	ginRouter := gin.Default()
	ginRouter.Handle("GET","/", func(context *gin.Context) {

		data:= make([]interface{},0)

		context.JSON(200,gin.H{
			"data":data,
		})
	})

	server := web.NewService(
		web.Address(":8000"),
		web.Handler(ginRouter),
	)
	server.Run()

}
