package api

import (
	"KubeMin-Cli/pkg/apiserver/domain/service"
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
	"net/http"
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
	var req apis.CreateWorkflowRequest
	if err := c.Bind(&req); err != nil {
		klog.Error(err)
		bcode.ReturnError(c, bcode.ErrWorkflowConfig)
		return
	}
	if err := validate.Struct(req); err != nil {
		bcode.ReturnError(c, err)
		return
	}
	ctx := c.Request.Context()
	resp, err := w.WorkflowService.CreateWorkflowTask(ctx, req)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
