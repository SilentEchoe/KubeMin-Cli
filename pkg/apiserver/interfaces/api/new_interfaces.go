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

func NewInitAPIBean() []interface{} {
	NewRegisterAPI(NewDemo())
	var beans []interface{}
	for i := range newRegisteredAPI {
		beans = append(beans, newRegisteredAPI[i])
	}
	return beans
}
