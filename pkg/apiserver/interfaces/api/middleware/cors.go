package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSOptions defines how cross-origin requests are handled.
type CORSOptions struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// CORS attaches the Access-Control headers and handles preflight requests.
func CORS(opts CORSOptions) gin.HandlerFunc {
	allowOrigins := normalizeList(opts.AllowOrigins)
	allowMethods := normalizeWithFallback(opts.AllowMethods, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	allowHeaders := normalizeWithFallback(opts.AllowHeaders, []string{"Content-Type", "Authorization", "Accept", "Origin", "X-Requested-With"})
	exposeHeaders := normalizeList(opts.ExposeHeaders)

	allowAllOrigins := hasItem(allowOrigins, "*")
	allowedMethodsHeader := strings.Join(allowMethods, ", ")
	allowedHeadersHeader := strings.Join(allowHeaders, ", ")
	exposedHeadersHeader := strings.Join(exposeHeaders, ", ")

	maxAge := ""
	if opts.MaxAge > 0 {
		maxAge = strconv.Itoa(int(opts.MaxAge / time.Second))
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Short-circuit non-CORS requests unless wildcard is configured
		if origin == "" && !allowAllOrigins {
			c.Next()
			return
		}

		switch {
		case allowAllOrigins && opts.AllowCredentials && origin != "":
			// Reflect origin to support credentials with wildcard config
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Add("Vary", "Origin")
		case allowAllOrigins:
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		case originAllowed(origin, allowOrigins):
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Add("Vary", "Origin")
		default:
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
			return
		}

		if opts.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if allowedMethodsHeader != "" {
			c.Writer.Header().Set("Access-Control-Allow-Methods", allowedMethodsHeader)
		}
		if allowedHeadersHeader != "" {
			c.Writer.Header().Set("Access-Control-Allow-Headers", allowedHeadersHeader)
		}
		if exposedHeadersHeader != "" {
			c.Writer.Header().Set("Access-Control-Expose-Headers", exposedHeadersHeader)
		}
		if maxAge != "" {
			c.Writer.Header().Set("Access-Control-Max-Age", maxAge)
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func originAllowed(origin string, allowed []string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(candidate, origin) {
			return true
		}
	}
	return false
}

func hasItem(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func normalizeWithFallback(values, fallback []string) []string {
	clean := normalizeList(values)
	if len(clean) == 0 {
		return normalizeList(fallback)
	}
	return clean
}

func normalizeList(values []string) []string {
	var out []string
	for _, v := range values {
		item := strings.TrimSpace(v)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}
