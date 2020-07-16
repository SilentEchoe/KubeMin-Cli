package Weblib

import (
	"LearningNotes-GoMicro/Services"
	"github.com/gin-gonic/gin"
)

func InitMiddleware(prodService Services.ProdService) gin.HandlerFunc  {
	return func(context *gin.Context) {
		context.Keys = make(map[string]interface{})
		context.Keys["prodservice"] = prodService //赋值
		context.Next()
	}
}