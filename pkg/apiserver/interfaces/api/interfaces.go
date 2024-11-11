package api

import (
	"github.com/emicklei/go-restful/v3"
)

// versionPrefix API version prefix.
var versionPrefix = "/api/v1"

type Interface interface {
	GetWebServiceRoute() *restful.WebService
}

var registeredAPI []Interface

// GetRegisteredAPI return all API handlers
func GetRegisteredAPI() []Interface {
	return registeredAPI
}

func InitAPIBean() []interface{} {
	return []interface{}{}
}
