package routers

import (
	"github.com/gin-gonic/gin"
	jwt "LearningNotes-GoMicro/middleware"

	"LearningNotes-GoMicro/routers/api"
)

func InitAutoCodeRouter(Router *gin.RouterGroup) {
	AutoCodeRouter := Router.Group("autoCode").
		Use(jwt.JWT())
	{
		AutoCodeRouter.POST("GetAuth", api.GetAuth) // 创建自动化代码

	}
}

