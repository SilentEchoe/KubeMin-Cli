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

type workflow struct {
	WorkflowService service.WorkflowService `inject:""`
}

func NewWorkflow() Interface {
	return &workflow{}
}

func (w *workflow) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/workflow", w.createWorkflow)
	group.POST("/workflow/exec", w.execWorkflowTask)
}

func (w *workflow) createWorkflow(c *gin.Context) {
	var req apis.CreateWorkflowRequest
	if err := c.Bind(&req); err != nil {
		klog.Error(err)
		bcode.ReturnError(c, bcode.ErrWorkflowConfig)
		return
	}
	normalizeWorkflowRequest(&req)
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

func (w *workflow) execWorkflowTask(c *gin.Context) {
	var req apis.ExecWorkflowRequest
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
	resp, err := w.WorkflowService.ExecWorkflowTask(ctx, req.WorkflowId)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func normalizeWorkflowRequest(req *apis.CreateWorkflowRequest) {
	req.Name = strings.ToLower(req.Name)
	req.Project = strings.ToLower(req.Project)
	for i := range req.Component {
		req.Component[i].Name = strings.ToLower(req.Component[i].Name)
		req.Component[i].NameSpace = strings.ToLower(req.Component[i].NameSpace)
	}
	for i := range req.Workflows {
		req.Workflows[i].Name = strings.ToLower(req.Workflows[i].Name)
	}
}
