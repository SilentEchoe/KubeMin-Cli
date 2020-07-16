package Weblib

import (
	"github.com/gin-gonic/gin"
)

func NewGinRouter()  {
	ginRouter := gin.Default()

	v1Group := ginRouter.Group("/v1")
	{
		v1Group.Handle("POST","/prods",GetProdsList)
	}
}