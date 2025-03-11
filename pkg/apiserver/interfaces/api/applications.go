package api

import (
	"KubeMin-Cli/pkg/apiserver/domain/service"
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"github.com/gin-gonic/gin"
	"net/http"
)

type applications struct {
	ApplicationService service.ApplicationsService `inject:""`
}

func (a *applications) RegisterRoutes(group *gin.RouterGroup) {
	// 应用列表
	group.GET("/applications", a.listApplications)
}

// NewApplications new applications manage
func NewApplications() Interface {
	return &applications{}
}

func (a *applications) listApplications(c *gin.Context) {
	apps, err := a.ApplicationService.ListApplications(c.Request.Context(), apis.ListApplicationOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, apps)
		return
	}
	c.JSON(http.StatusOK, apis.ListApplicationResponse{Applications: apps})

}
