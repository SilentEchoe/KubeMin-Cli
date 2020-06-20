package routers

import (


	// _ "LearningNotes-Go/docs"
	"github.com/gin-gonic/gin"
	//"github.com/swaggo/gin-swagger"
	//"github.com/swaggo/gin-swagger/swaggerFiles"

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
		//查询bin文件
		apiv1.POST("/test", v1.GetTestList)
	}

	return r
}
