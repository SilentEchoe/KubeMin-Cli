package bcode

import (
	"kubemin-cli/pkg/apiserver/infrastructure/datastore"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"net/http"
)

// Bcode business error code
type Bcode struct {
	HTTPCode     int32  `json:"-"`
	BusinessCode int32  `json:"business_code"`
	Message      string `json:"message"`
}

func (b Bcode) Error() string {
	return fmt.Sprintf("HTTPCode:%d BusinessCode:%d Message:%s", b.HTTPCode, b.BusinessCode, b.Message)
}

var bcodeMap map[int32]*Bcode

// NewBcode new business code
func NewBcode(httpCode, businessCode int32, message string) *Bcode {
	if bcodeMap == nil {
		bcodeMap = make(map[int32]*Bcode)
	}
	if _, exit := bcodeMap[businessCode]; exit {
		panic("bcode business code is exist")
	}
	bcode := &Bcode{HTTPCode: httpCode, BusinessCode: businessCode, Message: message}
	bcodeMap[businessCode] = bcode
	return bcode
}

// ReturnError Unified handling of all types of errors, generating a standard return structure.
func ReturnError(c *gin.Context, err error) {
	var bcode *Bcode
	if errors.As(err, &bcode) {
		c.JSON(int(bcode.HTTPCode), gin.H{
			"business_code": bcode.BusinessCode,
			"message":       bcode.Message,
		})
		return
	}

	if errors.Is(err, datastore.ErrRecordNotExist) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": err.Error(),
		})
		return
	}

	var validErr validator.ValidationErrors
	if errors.As(err, &validErr) {
		c.JSON(http.StatusBadRequest, Bcode{
			HTTPCode:     http.StatusBadRequest,
			BusinessCode: 400,
			Message:      err.Error(),
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"http_code":     http.StatusInternalServerError,
		"business_code": 500,
		"message":       err.Error(),
	})
	return
}

// ErrServer an unexpected mistake.
var ErrServer = NewBcode(500, 500, "The service has lapsed.")

// ErrForbidden check user perms failure
var ErrForbidden = NewBcode(403, 403, "403 Forbidden")

// ErrUnauthorized check user auth failure
var ErrUnauthorized = NewBcode(401, 401, "401 Unauthorized")

// ErrNotFound the request resource is not found
var ErrNotFound = NewBcode(404, 404, "404 Not Found")

// ErrUpstreamNotFound the proxy upstream is not found
var ErrUpstreamNotFound = NewBcode(502, 502, "Upstream not found")
