package routers

import (
	"github.com/gin-gonic/gin"

	"LearningNotes-GoMicro/routers/api/v1"
)

func InitRouter() *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger())

	r.Use(gin.Recovery())

	//r.GET("/auth", api.GetAuth)
	//r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	apiv1 := r.Group("/api/v1")
	//jwt.JWT()
	apiv1.Use()
	{
		//获取通知列表
		apiv1.GET("/notices", v1.GetNotices)
		// 新增通知
		apiv1.POST("/notices", v1.AddNotices)
	}

	return r
}
