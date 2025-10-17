package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Gzip is a minimal gzip middleware for gin that compresses responses when
// the client advertises gzip support and the response isn't already encoded.
func Gzip() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only gzip for clients that accept it
		ae := c.GetHeader("Accept-Encoding")
		if !strings.Contains(ae, "gzip") {
			c.Next()
			return
		}

		// Skip if already set by an upstream handler
		if c.Writer.Header().Get("Content-Encoding") != "" {
			c.Next()
			return
		}

		// Wrap the ResponseWriter
		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()

		w := &gzipWriter{ResponseWriter: c.Writer, Writer: gz}
		w.Header().Set("Content-Encoding", "gzip")
		// Per RFC, vary on Accept-Encoding
		w.Header().Add("Vary", "Accept-Encoding")

		c.Writer = w
		c.Next()
	}
}

type gzipWriter struct {
	gin.ResponseWriter
	Writer io.Writer
}

func (w *gzipWriter) Write(data []byte) (int, error) {
	// If status not written yet, ensure a default OK
	if !w.Written() {
		w.WriteHeader(http.StatusOK)
	}
	return w.Writer.Write(data)
}
