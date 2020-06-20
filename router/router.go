package routers

import (
	"LearningNotes-Go/pkg/setting"

	_ "LearningNotes-Go/docs"
	"github.com/gin-gonic/gin"
	"github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"

	"LearningNotes-Go/routers/api/v1"
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
		//获取moduleName
		apiv1.GET("/moduleNames", v1.GetModelNames)

		//查询型号类型
		apiv1.GET("/moduleTypes", v1.GetModelTypes)

		//查询bin文件
		apiv1.POST("/moduleBins", v1.GetModelBins)

		//查询配置文件
		apiv1.GET("/configFiles", v1.GetConfigFiles)
	}

	return r
}
