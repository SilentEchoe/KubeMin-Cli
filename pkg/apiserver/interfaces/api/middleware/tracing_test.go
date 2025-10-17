package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"k8s.io/klog/v2"
)

func TestLoggingMiddleware(t *testing.T) {
	// 1. Setup in-memory exporter and tracer provider
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)

	// 2. Capture klog output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	klog.SetOutput(w)

	// 3. Setup Gin server with middlewares
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(otelgin.Middleware("test-service"))
	router.Use(Logging())
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// 4. Perform a request
	writer := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/ping", nil)
	router.ServeHTTP(writer, req)

	// 5. Restore stderr and read captured logs
	w.Close()
	os.Stderr = oldStderr
	klog.SetOutput(os.Stderr)
	var logBuf bytes.Buffer
	io.Copy(&logBuf, r)
	logOutput := logBuf.String()

	// 6. Assertions
	// Assert response
	assert.Equal(t, http.StatusOK, writer.Code)

	// Assert that a span was created
	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	span := spans[0]

	// Assert that the log contains the correct traceID and other details
	traceID := span.SpanContext.TraceID().String()
	assert.True(t, strings.Contains(logOutput, traceID), "Log output should contain the traceID")
	assert.True(t, strings.Contains(logOutput, `status=200`), "Log output should contain the status code")
	assert.True(t, strings.Contains(logOutput, `path="/ping"`), "Log output should contain the path")

	t.Logf("Captured log: %s", logOutput)
	t.Logf("Captured traceID: %s", traceID)
}
