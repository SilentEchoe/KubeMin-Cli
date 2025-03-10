package api

import (
	"KubeMin-Cli/pkg/apiserver/domain/service"
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
)

type applications struct {
	ApplicationService service.ApplicationsService `inject:""`
}

// NewApplications new applications manage
func NewApplications() Interface {
	return &applications{}
}

func (a *applications) GetWebServiceRoute() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path(versionPrefix+"/applications").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML).
		Doc("api for applications manage")

	tags := []string{"applications"}

	ws.Route(ws.GET("/").To(a.listApplications).
		Doc("list all applications").
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Param(ws.QueryParameter("query", "Fuzzy search based on name or description").DataType("string")).
		Param(ws.QueryParameter("project", "search base on project name").DataType("string")).
		Param(ws.QueryParameter("env", "search base on env name").DataType("string")).
		Param(ws.QueryParameter("targetName", "Name of the applications delivery target").DataType("string")).
		// This api will filter the app by user's permissions
		// Filter(c.RbacService.CheckPerm("applications", "list")).
		Returns(200, "OK", apis.ListApplicationResponse{}).
		Returns(400, "Bad Request", bcode.Bcode{}).
		Writes(apis.ListApplicationResponse{}))

	return ws
}

func (a *applications) listApplications(req *restful.Request, res *restful.Response) {
	//fmt.Println("listApplications")
	//res.WriteEntity("listApplications")
	apps, err := a.ApplicationService.ListApplications(req.Request.Context(), apis.ListApplicationOptions{})
	if err != nil {
		bcode.ReturnError(req, res, err)
	}
	if err := res.WriteEntity(apis.ListApplicationResponse{Applications: apps}); err != nil {
		bcode.ReturnError(req, res, err)
		return
	}
}
