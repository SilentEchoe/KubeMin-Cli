package api

import (
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
	"fmt"
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
)

type application struct {
}

// NewApplication new application manage
func NewApplication() Interface {
	return &application{}
}

func (a *application) GetWebServiceRoute() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path(versionPrefix+"/applications").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML).
		Doc("api for application manage")

	tags := []string{"application"}

	ws.Route(ws.GET("/").To(a.listApplications).
		Doc("list all applications").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.QueryParameter("query", "Fuzzy search based on name or description").DataType("string")).
		Param(ws.QueryParameter("project", "search base on project name").DataType("string")).
		Param(ws.QueryParameter("env", "search base on env name").DataType("string")).
		Param(ws.QueryParameter("targetName", "Name of the application delivery target").DataType("string")).
		// This api will filter the app by user's permissions
		// Filter(c.RbacService.CheckPerm("application", "list")).
		Returns(200, "OK", apis.ListApplicationResponse{}).
		Returns(400, "Bad Request", bcode.Bcode{}).
		Writes(apis.ListApplicationResponse{}))

	return ws
}

func (a *application) listApplications(req *restful.Request, res *restful.Response) {
	fmt.Println("listApplications")
	res.WriteEntity("listApplications")
}
