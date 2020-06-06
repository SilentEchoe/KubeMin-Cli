package main

import (
	"github.com/gin-gonic/gin"
	"github.com/micro/go-micro/web"
)

func main()  {

	ginRouter := gin.Default()
	ginRouter.Handle("GET","/user", func(context *gin.Context) {
		context.String(200,"user api")
	})

	server := web.NewService(
		web.Address(":8001"),
		web.Handler(ginRouter),
		)
	server.Run()

}
