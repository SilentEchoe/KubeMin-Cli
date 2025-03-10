package api

import (
	apisv1 "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"github.com/emicklei/go-restful/v3"
	"net/http"
)

var versionPrefix = "/api/v1"

// GetAPIPrefix return the prefix of the api route path
func GetAPIPrefix() []string {
	return []string{versionPrefix, viewPrefix, "/v1"}
}

// viewPrefix the path prefix for view page
var viewPrefix = "/view"

// Interface the API should define the http route
type Interface interface {
	GetWebServiceRoute() *restful.WebService
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

func returns200(b *restful.RouteBuilder) {
	b.Returns(http.StatusOK, "OK", apisv1.SimpleResponse{Status: "ok"})
}

func returns500(b *restful.RouteBuilder) {
	b.Returns(http.StatusInternalServerError, "Bummer, something went wrong", nil)
}

// InitAPIBean inits all API handlers, pass in the required parameter object.
// It can be implemented using the idea of dependency injection.
func InitAPIBean() []interface{} {
	RegisterAPI(NewApplications())
	var beans []interface{}
	for i := range registeredAPI {
		beans = append(beans, registeredAPI[i])
	}
	return beans
}
