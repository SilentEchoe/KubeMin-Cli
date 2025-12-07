package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/service"
	assembler "KubeMin-Cli/pkg/apiserver/interfaces/api/assembler/v1"
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

func (app *applications) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/applications", app.listApplications)
	group.GET("/applications/templates", app.listTemplateApplications)
	group.POST("/applications", app.createApplications)
	group.GET("/applications/:appID/workflows", app.listApplicationWorkflows)
	group.GET("/applications/:appID/components", app.listApplicationComponents)
	group.PUT("/applications/:appID/workflow", app.updateApplicationWorkflow)
	group.DELETE("/applications/:appID/resources", app.deleteApplicationResources)
	group.POST("/applications/:appID/workflow/exec", app.execApplicationWorkflow)
	group.POST("/applications/:appID/workflow/cancel", app.cancelApplicationWorkflow)
	group.GET("/workflow/tasks/:taskID/status", app.getWorkflowTaskStatus)
	// 版本更新接口
	group.POST("/applications/:appID/version", app.updateVersion)
}

func (app *applications) createApplications(c *gin.Context) {
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
	resp, err := app.ApplicationService.CreateApplications(ctx, req)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (app *applications) listApplications(c *gin.Context) {
	apps, err := app.ApplicationService.ListApplications(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apps)
		return
	}
	c.JSON(http.StatusOK, apis.ListApplicationResponse{Applications: apps})
}

func (app *applications) listTemplateApplications(c *gin.Context) {
	apps, err := app.ApplicationService.ListTemplateApplications(c.Request.Context())
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, apis.ListApplicationResponse{Applications: apps})
}

func (app *applications) listApplicationWorkflows(c *gin.Context) {
	appID := strings.TrimSpace(c.Param("appID"))
	if appID == "" {
		bcode.ReturnError(c, bcode.ErrApplicationNotExist)
		return
	}
	ctx := c.Request.Context()
	workflows, err := app.ApplicationService.ListApplicationWorkflows(ctx, appID)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	resp := make([]*apis.ApplicationWorkflow, 0, len(workflows))
	for _, wf := range workflows {
		if wf == nil {
			continue
		}
		dto, err := assembler.ConvertWorkflowModelToDTO(wf)
		if err != nil {
			klog.Errorf("convert workflow dto failed appID=%s workflowID=%s: %v", appID, wf.ID, err)
			bcode.ReturnError(c, err)
			return
		}
		if dto != nil {
			resp = append(resp, dto)
		}
	}
	c.JSON(http.StatusOK, apis.ListApplicationWorkflowsResponse{Workflows: resp})
}

func (app *applications) listApplicationComponents(c *gin.Context) {
	appID := strings.TrimSpace(c.Param("appID"))
	if appID == "" {
		bcode.ReturnError(c, bcode.ErrApplicationNotExist)
		return
	}
	ctx := c.Request.Context()
	components, err := app.ApplicationService.ListApplicationComponents(ctx, appID)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	resp := make([]*apis.ApplicationComponent, 0, len(components))
	for _, comp := range components {
		if comp == nil {
			continue
		}
		dto, err := assembler.ConvertComponentModelToDTO(comp)
		if err != nil {
			klog.Errorf("convert component dto failed appID=%s component=%s: %v", appID, comp.Name, err)
			bcode.ReturnError(c, err)
			return
		}
		if dto != nil {
			resp = append(resp, dto)
		}
	}
	c.JSON(http.StatusOK, apis.ListApplicationComponentsResponse{Components: resp})
}

func (app *applications) deleteApplicationResources(c *gin.Context) {
	appID := strings.TrimSpace(c.Param("appID"))
	if appID == "" {
		bcode.ReturnError(c, bcode.ErrApplicationNotExist)
		return
	}
	resp, err := app.ApplicationService.CleanupApplicationResources(c.Request.Context(), appID)
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

func (app *applications) updateApplicationWorkflow(c *gin.Context) {
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
	resp, err := app.ApplicationService.UpdateApplicationWorkflow(ctx, appID, req)
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

func (app *applications) execApplicationWorkflow(c *gin.Context) {
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
	resp, err := app.WorkflowService.ExecWorkflowTaskForApp(ctx, appID, req.WorkflowID)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (app *applications) cancelApplicationWorkflow(c *gin.Context) {
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
	if err := app.WorkflowService.CancelWorkflowTaskForApp(ctx, appID, user, req.TaskID, req.Reason); err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, apis.CancelWorkflowResponse{TaskID: req.TaskID, Status: string(config.StatusCancelled)})
}

func (app *applications) getWorkflowTaskStatus(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("taskID"))
	if taskID == "" {
		bcode.ReturnError(c, bcode.ErrWorkflowTaskNotExist)
		return
	}
	ctx := c.Request.Context()
	resp, err := app.WorkflowService.GetTaskStatus(ctx, taskID)
	if err != nil {
		bcode.ReturnError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// updateVersion 更新应用版本
func (app *applications) updateVersion(c *gin.Context) {
	appID := strings.TrimSpace(c.Param("appID"))
	if appID == "" {
		bcode.ReturnError(c, bcode.ErrApplicationNotExist)
		return
	}

	var req apis.UpdateVersionRequest
	if err := c.Bind(&req); err != nil {
		klog.Error(err)
		bcode.ReturnError(c, bcode.ErrApplicationConfig)
		return
	}

	// 规范化组件名称
	for i := range req.Components {
		req.Components[i].Name = strings.ToLower(strings.TrimSpace(req.Components[i].Name))
	}

	if err := validate.Struct(req); err != nil {
		bcode.ReturnError(c, err)
		return
	}

	ctx := c.Request.Context()
	klog.Infof("update version request received appID=%s version=%s strategy=%s components=%d",
		appID, req.Version, req.Strategy, len(req.Components))

	resp, err := app.ApplicationService.UpdateVersion(ctx, appID, req)
	if err != nil {
		klog.Errorf("update version failed appID=%s error=%v", appID, err)
		bcode.ReturnError(c, err)
		return
	}

	klog.Infof("update version succeeded appID=%s newVersion=%s taskID=%s",
		appID, resp.Version, resp.TaskID)
	c.JSON(http.StatusOK, resp)
}
