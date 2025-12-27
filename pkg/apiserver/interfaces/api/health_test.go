package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	msg "kubemin-cli/pkg/apiserver/infrastructure/messaging"
)

type mockHealthQueue struct {
	statsError error
}

func (m *mockHealthQueue) EnsureGroup(ctx context.Context, group string) error { return nil }
func (m *mockHealthQueue) Enqueue(ctx context.Context, payload []byte) (string, error) {
	return "", nil
}
func (m *mockHealthQueue) ReadGroup(ctx context.Context, group, consumer string, count int, block time.Duration) ([]msg.Message, error) {
	return nil, nil
}
func (m *mockHealthQueue) Ack(ctx context.Context, group string, ids ...string) error { return nil }
func (m *mockHealthQueue) AutoClaim(ctx context.Context, group, consumer string, minIdle time.Duration, count int) ([]msg.Message, error) {
	return nil, nil
}
func (m *mockHealthQueue) Close(ctx context.Context) error { return nil }
func (m *mockHealthQueue) Stats(ctx context.Context, group string) (int64, int64, error) {
	if m.statsError != nil {
		return 0, 0, m.statsError
	}
	return 10, 5, nil
}

func TestHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &health{}
	r := gin.New()
	r.GET("/health", h.healthCheck)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "healthy")
}

func TestHealthzCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &health{}
	r := gin.New()
	r.GET("/healthz", h.healthCheck)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "healthy")
}

func TestReadinessCheckWithHealthyQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &health{
		Queue: &mockHealthQueue{},
	}
	r := gin.New()
	r.GET("/ready", h.readinessCheck)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "ready")
}

func TestReadinessCheckWithUnhealthyQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &health{
		Queue: &mockHealthQueue{statsError: errors.New("connection refused")},
	}
	r := gin.New()
	r.GET("/ready", h.readinessCheck)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
	require.Contains(t, resp.Body.String(), "not ready")
	require.Contains(t, resp.Body.String(), "queue connection failed")
}

func TestReadinessCheckWithNoopQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &health{
		Queue: &msg.NoopQueue{},
	}
	r := gin.New()
	r.GET("/ready", h.readinessCheck)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	// NoopQueue should always be considered ready
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "ready")
}

func TestReadinessCheckWithNilQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &health{
		Queue: nil,
	}
	r := gin.New()
	r.GET("/ready", h.readinessCheck)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	// Nil queue should be considered ready (no dependency)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "ready")
}

func TestHealthGetName(t *testing.T) {
	h := &health{}
	require.Equal(t, "health", h.GetName())
}
