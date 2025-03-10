package api

import (
	"github.com/gin-gonic/gin"
)

// NewInterface the API should define the http route
type NewInterface interface {
	RegisterRoutes(group *gin.RouterGroup)
}

var newRegisteredAPI []NewInterface

// NewRegisterAPI register API handler
func NewRegisterAPI(ws NewInterface) {
	newRegisteredAPI = append(newRegisteredAPI, ws)
}

func NewGetRegisteredAPI() []NewInterface {
	return newRegisteredAPI
}

func SetupRoutes(router *gin.Engine) {
	// 创建API路由组
	apiGroup := router.Group(versionPrefix)

	// 注册所有API
	for _, api := range newRegisteredAPI {
		api.RegisterRoutes(apiGroup)
	}

	v1Group := router.Group("/demo")
	for _, api := range newRegisteredAPI {
		api.RegisterRoutes(v1Group)
	}

}

func NewInitAPIBean() []interface{} {
	NewRegisterAPI(NewDemo())
	var beans []interface{}
	for i := range newRegisteredAPI {
		beans = append(beans, newRegisteredAPI[i])
	}
	return beans
}
