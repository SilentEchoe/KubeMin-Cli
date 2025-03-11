package bcode

import (
	"KubeMin-Cli/pkg/apiserver/infrastructure/datastore"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"net/http"
)

// Bcode business error code
type Bcode struct {
	HTTPCode     int32  `json:"-"`
	BusinessCode int32  `json:"BusinessCode"`
	Message      string `json:"Message"`
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
		c.JSON(int(bcode.HTTPCode), err)
		return
	}

	if errors.Is(err, datastore.ErrRecordNotExist) {
		c.JSON(http.StatusNotFound, err)
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

	c.JSON(http.StatusInternalServerError, Bcode{
		HTTPCode:     http.StatusInternalServerError,
		BusinessCode: 500,
		Message:      err.Error(),
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
