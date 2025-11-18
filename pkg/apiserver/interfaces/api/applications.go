package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	apis "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
	"KubeMin-Cli/pkg/apiserver/utils/bcode"
)

type applications struct {
	ApplicationService service.ApplicationsService `inject:""`
	WorkflowService    service.WorkflowService     `inject:""`
}

// NewApplications new applications manage
func NewApplications() Interface {
	return &applications{}
}

func (a *applications) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/applications", a.listApplications)
	group.POST("/applications", a.createApplications)
	group.DELETE("/applications/:appID/resources", a.deleteApplicationResources)
	group.PUT("/applications/:appID/workflow", a.updateApplicationWorkflow)
	group.POST("/applications/:appID/workflow/exec", a.execApplicationWorkflow)
	group.POST("/applications/:appID/workflow/cancel", a.cancelApplicationWorkflow)
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

func (a *applications) updateApplicationWorkflow(c *gin.Context) {
	appID := strings.TrimSpace(c.Param("appID"))
	if appID == "" {
		bcode.ReturnError(c, bcode.ErrApplicationNotExist)
		return
	}
	var req apis.UpdateApplicationWorkflowRequest
	if err := c.Bind(&req); err != nil {
		klog.Error(err)
		bcode.ReturnError(c, bcode.ErrWorkflowConfig)
		return
	}
	normalizeWorkflowSteps(req.Workflow)
	if err := validate.Struct(req); err != nil {
		bcode.ReturnError(c, err)
		return
	}
	ctx := c.Request.Context()
	klog.Infof("update workflow request received appID=%s workflowId=%s name=%s", appID, req.WorkflowID, req.Name)
	resp, err := a.ApplicationService.UpdateApplicationWorkflow(ctx, appID, req)
	if err != nil {
		klog.Errorf("update workflow failed appID=%s workflowId=%s error=%v", appID, req.WorkflowID, err)
		bcode.ReturnError(c, err)
		return
	}
	klog.Infof("update workflow succeeded appID=%s workflowId=%s", appID, resp.WorkflowID)
	c.JSON(http.StatusOK, resp)
}

func normalizeWorkflowSteps(steps []apis.CreateWorkflowStepRequest) {
	for i := range steps {
		steps[i].Name = strings.ToLower(steps[i].Name)
		for j := range steps[i].Components {
			steps[i].Components[j] = strings.ToLower(steps[i].Components[j])
		}
		for j := range steps[i].Properties.Policies {
			steps[i].Properties.Policies[j] = strings.ToLower(steps[i].Properties.Policies[j])
		}
		for j := range steps[i].SubSteps {
			steps[i].SubSteps[j].Name = strings.ToLower(steps[i].SubSteps[j].Name)
			for k := range steps[i].SubSteps[j].Properties.Policies {
				steps[i].SubSteps[j].Properties.Policies[k] = strings.ToLower(steps[i].SubSteps[j].Properties.Policies[k])
			}
			for k := range steps[i].SubSteps[j].Components {
				steps[i].SubSteps[j].Components[k] = strings.ToLower(steps[i].SubSteps[j].Components[k])
			}
		}
	}
}

func (a *applications) execApplicationWorkflow(c *gin.Context) {
	appID := strings.TrimSpace(c.Param("appID"))
	if appID == "" {
		bcode.ReturnError(c, bcode.ErrApplicationNotExist)
		return
	}
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
	resp, err := a.WorkflowService.ExecWorkflowTaskForApp(ctx, appID, req.WorkflowID)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (a *applications) cancelApplicationWorkflow(c *gin.Context) {
	appID := strings.TrimSpace(c.Param("appID"))
	if appID == "" {
		bcode.ReturnError(c, bcode.ErrApplicationNotExist)
		return
	}
	var req apis.CancelWorkflowRequest
	if err := c.Bind(&req); err != nil {
		klog.Error(err)
		bcode.ReturnError(c, bcode.ErrWorkflowConfig)
		return
	}
	if err := validate.Struct(req); err != nil {
		bcode.ReturnError(c, err)
		return
	}
	user := req.User
	if user == "" {
		user = config.DefaultTaskRevoker
	}
	ctx := c.Request.Context()
	if err := a.WorkflowService.CancelWorkflowTaskForApp(ctx, appID, user, req.TaskID, req.Reason); err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, apis.CancelWorkflowResponse{TaskID: req.TaskID, Status: string(config.StatusCancelled)})
}
