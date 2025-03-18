package api

import (
	"KubeMin-Cli/pkg/apiserver/domain/service"
	"github.com/gin-gonic/gin"
)

type workflow struct {
	WorkflowService service.WorkflowService `inject:""`
}

func NewWorkflow() Interface {
	return &workflow{}
}

func (w *workflow) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/workflow", w.createWorkflow)
}

func (w *workflow) createWorkflow(c *gin.Context) {

}
