package main

import (
	"github.com/gin-contrib/cors"
)

func main() {
	svc := InitializeApp()
	svc.Use(cors.Default())
	//注册服务

	svc.Run(":8080")
}
