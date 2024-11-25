package api

import (
	"github.com/emicklei/go-restful/v3"
)

// versionPrefix API version prefix.
var versionPrefix = "/api/v1"

// GetAPIPrefix return the prefix of the api route path
func GetAPIPrefix() []string {
	return []string{versionPrefix, viewPrefix, "/v1"}
}

// viewPrefix the path prefix for view page
var viewPrefix = "/view"

type Interface interface {
	GetWebServiceRoute() *restful.WebService
}

var registeredAPI []Interface

// GetRegisteredAPI return all API handlers
func GetRegisteredAPI() []Interface {
	return registeredAPI
}

func InitAPIBean() []interface{} {
	RegisterAPI(NewApplication())
	var beans []interface{}
	for i := range registeredAPI {
		beans = append(beans, registeredAPI[i])
	}
	//beans = append(beans, NewWorkflow())
	return beans
}

// RegisterAPI register API handler
func RegisterAPI(ws Interface) {
	registeredAPI = append(registeredAPI, ws)
}
