package api

import (
	"github.com/gin-gonic/gin"
)

var versionPrefix = "/api/v1"

// GetAPIPrefix return the prefix of the api route path
func GetAPIPrefix() []string {
	return []string{versionPrefix}
}

// The Interface API should define the http route
type Interface interface {
	RegisterRoutes(group *gin.RouterGroup)
}

var registeredAPI []Interface

// RegisterAPI register API handler
func RegisterAPI(ws Interface) {
	registeredAPI = append(registeredAPI, ws)
}

// GetRegisteredAPI return all API handlers
func GetRegisteredAPI() []Interface {
	return registeredAPI
}

// InitAPIBean inits all API handlers, pass in the required parameter object.
// It can be implemented using the idea of dependency injection.
func InitAPIBean() []interface{} {
	RegisterAPI(NewApplications())
	RegisterAPI(NewWorkflow())
	var beans []interface{}
	for i := range registeredAPI {
		beans = append(beans, registeredAPI[i])
	}
	return beans
}
