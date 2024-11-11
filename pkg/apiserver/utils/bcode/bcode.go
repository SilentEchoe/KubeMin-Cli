package bcode

import (
	"errors"
	"github.com/emicklei/go-restful/v3"
	"k8s.io/klog/v2"
	"net/http"
)

// Bcode business error code
type Bcode struct {
	HTTPCode     int32  `json:"-"`
	BusinessCode int32  `json:"BusinessCode"`
	Message      string `json:"Message"`
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

// ReturnHTTPError Unified handling of all types of errors, generating a standard return structure.
func ReturnHTTPError(req *http.Request, res http.ResponseWriter, err error) {
	restRes := restful.NewResponse(res)
	restRes.SetRequestAccepts(restful.MIME_JSON)
	ReturnError(restful.NewRequest(req), restRes, err)
}

// ReturnError Unified handling of all types of errors, generating a standard return structure.
func ReturnError(req *restful.Request, res *restful.Response, err error) {
	var bcode *Bcode
	if errors.As(err, &bcode) {
		if err := res.WriteHeaderAndEntity(int(bcode.HTTPCode), err); err != nil {
			klog.Errorf("write entity failure %s", err.Error())
		}
		return
	}

	var restfulerr restful.ServiceError
	if errors.As(err, &restfulerr) {
		if err := res.WriteHeaderAndEntity(restfulerr.Code, Bcode{HTTPCode: int32(restfulerr.Code), BusinessCode: int32(restfulerr.Code), Message: restfulerr.Message}); err != nil {
			klog.Errorf("write entity failure %s", err.Error())
		}
		return
	}

	//klog.Errorf("Business exceptions, error message: %s, path:%s method:%s", err.Error(), utils.Sanitize(req.Request.URL.String()), req.Request.Method)
	if err := res.WriteHeaderAndEntity(500, Bcode{HTTPCode: 500, BusinessCode: 500, Message: err.Error()}); err != nil {
		klog.Errorf("write entity failure %s", err.Error())
	}
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
