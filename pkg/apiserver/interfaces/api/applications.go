package api

import (
	"KubeMin-Cli/pkg/apiserver/domain/model"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
	"github.com/gin-gonic/gin"
	"net/http"
)

type applications struct {
	ApplicationService service.ApplicationsService `inject:""`
}

// NewApplications new applications manage
func NewApplications() Interface {
	return &applications{}
}

func (a *applications) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/applications", a.listApplications)
	group.POST("/applications/deploy", a.deployApplication)

}

func (a *applications) listApplications(c *gin.Context) {
	apps, err := a.ApplicationService.ListApplications(c.Request.Context(), apis.ListApplicationOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, apps)
		return
	}
	c.JSON(http.StatusOK, apis.ListApplicationResponse{Applications: apps})
}

func (a *applications) deployApplication(c *gin.Context) {
	app := c.Request.Context().Value(&apis.CtxKeyApplication).(*model.Applications)
	var req apis.ApplicationsDeployRequest
	if err := c.Bind(req); err != nil {
		bcode.ReturnError(c, bcode.ErrApplicationConfig)
	}
	// 验证入参是否正确
	if err := validate.Struct(req); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	resp, err := a.ApplicationService.Deploy(ctx, app, req)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
