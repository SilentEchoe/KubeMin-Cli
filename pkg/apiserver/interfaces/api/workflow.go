package api

import (
	"net/http"
	"strings"

	"kubemin-cli/pkg/apiserver/domain/service"
	apis "kubemin-cli/pkg/apiserver/interfaces/api/dto/v1"
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
	group.POST("/workflow/exec", w.execWorkflowTask)
	group.POST("/workflow/cancel", w.cancelWorkflowTask)
}

func (w *workflow) createWorkflow(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "custom workflow execution API not implemented yet",
	})
}

func (w *workflow) execWorkflowTask(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "custom workflow execution API not implemented yet",
	})
}

func (w *workflow) cancelWorkflowTask(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "custom workflow cancellation API not implemented yet",
	})
}

func normalizeWorkflowRequest(req *apis.CreateWorkflowRequest) {
	req.Name = strings.ToLower(req.Name)
	req.Project = strings.ToLower(req.Project)
	for i := range req.Component {
		req.Component[i].Name = strings.ToLower(req.Component[i].Name)
		req.Component[i].Namespace = strings.ToLower(req.Component[i].Namespace)
	}
	for i := range req.Workflows {
		req.Workflows[i].Name = strings.ToLower(req.Workflows[i].Name)
	}
}
