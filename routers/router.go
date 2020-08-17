package routers

import (

	_ "LearningNotes-GoMicro/docs"
	middleware "LearningNotes-GoMicro/middleware"
	"LearningNotes-GoMicro/pkg/upload"
	"LearningNotes-GoMicro/routers/api"

	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"
	"net/http"

	"github.com/gin-gonic/gin"

	"LearningNotes-GoMicro/routers/api/v1"
)

func InitRouter() *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger())

	r.Use(gin.Recovery())

	r.GET("/auth", api.GetAuth)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.StaticFS("/upload/images", http.Dir(upload.GetImageFullPath()))
	r.POST("/upload", api.UploadImage)
	apiv1 := r.Group("/api/v1")
	apiv1.Use(middleware.JWT())
	{
		// 从redis 获取通知列表
		apiv1.GET("/notice",v1.GetNoticesByRedis)
		//获取通知列表
		//apiv1.GET("/notices", v1.GetNotices)
		// 新增通知
		apiv1.POST("/notices", v1.AddNotices)

		// 测试通告分页
		apiv1.GET("/test",v1.GetNoticesPage)

		apiv1.GET("/tests",v1.GetNoticesPageTest)
	}

	return r
}

/*func Routers() *gin.Engine {
	var Router = gin.Default()
	//Router.Use(middleware.Cors())
	Router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	// 方便统一添加路由组前缀 多服务器上线使用
	ApiGroup := Router.Group("")
	routers.InitAutoCodeRouter(ApiGroup)                  // 注册用户路由
	return Router

}*/