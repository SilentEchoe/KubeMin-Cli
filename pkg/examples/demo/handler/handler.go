package handler

import (
	"KubeMin-Cli/pkg/examples/demo/service"
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
)

type ApplicationHandler struct {
	svc service.IApplications
}

func (h *ApplicationHandler) RegisterRoutes(c *gin.Engine) {
	c.GET("/applications", h.Get)
}

func (h *ApplicationHandler) Get(c *gin.Context) {
	// get applications
	content := h.svc.Get(context.Background())
	c.String(http.StatusOK, content)
}

func NewApplicationHandler(svc service.IApplications) *ApplicationHandler {
	return &ApplicationHandler{svc: svc}
}
