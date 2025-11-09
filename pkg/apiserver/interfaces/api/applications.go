package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/domain/service"
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
)

type applications struct {
	ApplicationService service.ApplicationsService `inject:""`
}

// NewApplications new applications manage
func NewApplications() Interface {
	return &applications{}
}

func (a *applications) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/applications", a.createApplications)
	group.GET("/applications", a.listApplications)
	group.DELETE("/applications/:appID/resources", a.deleteApplicationResources)
}

func (a *applications) createApplications(c *gin.Context) {
	var req apis.CreateApplicationsRequest
	if err := c.Bind(&req); err != nil {
		klog.Error(err)
		bcode.ReturnError(c, bcode.ErrApplicationConfig)
		return
	}

	if err := validate.Struct(req); err != nil {
		bcode.ReturnError(c, err)
		return
	}
	ctx := c.Request.Context()
	resp, err := a.ApplicationService.CreateApplications(ctx, req)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (a *applications) listApplications(c *gin.Context) {
	apps, err := a.ApplicationService.ListApplications(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apps)
		return
	}
	c.JSON(http.StatusOK, apis.ListApplicationResponse{Applications: apps})
}

func (a *applications) deleteApplicationResources(c *gin.Context) {
	appID := strings.TrimSpace(c.Param("appID"))
	if appID == "" {
		bcode.ReturnError(c, bcode.ErrApplicationNotExist)
		return
	}
	resp, err := a.ApplicationService.CleanupApplicationResources(c.Request.Context(), appID)
	if err != nil {
		if resp != nil {
			c.JSON(http.StatusInternalServerError, resp)
			return
		}
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
