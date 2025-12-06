package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	msg "KubeMin-Cli/pkg/apiserver/infrastructure/messaging"
)

func init() {
	RegisterAPI(&health{})
}

// health provides health check endpoints for Kubernetes probes.
type health struct {
	Queue msg.Queue `inject:"queue"`
}

// GetName returns the API name for registration.
func (h *health) GetName() string {
	return "health"
}

// RegisterRoutes registers health check endpoints.
func (h *health) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/health", h.healthCheck)
	group.GET("/healthz", h.healthCheck)
	group.GET("/ready", h.readinessCheck)
	group.GET("/readyz", h.readinessCheck)
}

// healthCheck returns a simple health status (liveness probe).
// This endpoint always returns OK if the server is running.
func (h *health) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// readinessCheck checks if the server is ready to accept traffic.
// It verifies connectivity to dependencies like the message queue.
func (h *health) readinessCheck(c *gin.Context) {
	ctx := c.Request.Context()

	// Check queue connectivity
	if h.Queue != nil {
		if _, ok := h.Queue.(*msg.NoopQueue); !ok {
			// Real queue - check connectivity
			if _, _, err := h.Queue.Stats(ctx, "workflow-workers"); err != nil {
				klog.V(4).Infof("readiness check failed: queue stats error: %v", err)
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"status": "not ready",
					"error":  "queue connection failed",
				})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}


