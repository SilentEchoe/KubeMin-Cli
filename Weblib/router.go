package Weblib

import (
	"LearningNotes-GoMicro/Services"
	"github.com/gin-gonic/gin"
	"net/http"
)

func NewGinRouter(prodService Services.ProdService) http.Handler {
	ginRouter := gin.Default()
	ginRouter.Use(InitMiddleware(prodService))
	v1Group := ginRouter.Group("/v1")
	{
		v1Group.Handle("POST","/prods",GetProdsList)
	}
	return nil
}