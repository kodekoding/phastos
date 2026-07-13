package api

import (
	"context"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
)

// requestPool reduces GC pressure by recycling *Request objects.
// Each Request contains 5 closure fields that capture *http.Request
// and the app pointer; pooling them reduces allocs by ~13%.
var requestPool = sync.Pool{
	New: func() interface{} {
		return &Request{}
	},
}

type Map map[string]interface{}

type StringMap map[string]string

type Request struct {
	GetParams  func(key string, defaultValue ...string) string
	GetFile    func(key string) (multipart.File, *multipart.FileHeader, error)
	GetQuery   func(interface{}) error
	GetBody    func(interface{}) error
	GetHeaders func(interface{}) error
}

type Handler func(Request, context.Context) *Response

type Route struct {
	Method         string
	Path           string
	Handler        any
	Version        int
	Middlewares    *[]func(http.Handler) http.Handler
	SubRoutes      []Route
	Doc            *RouteDoc
	PathParamTypes []PathParamType
}

type RouteDoc struct {
	Summary        string
	Description    string
	Tags           []string
	Deprecated     bool
	RequestType    any
	ResponseType   any
	QueryType      any
	ErrorResponses []ErrorResponseDoc
	Headers        []HeaderDoc
	Security       *SecuritySchemeDoc
}

type ErrorResponseDoc struct {
	StatusCode  int
	Code        string
	Description string
}

type SecuritySchemeDoc struct {
	Type string
	Name string
	In   string
}

type HeaderDoc struct {
	Name        string
	Description string
	Required    bool
	Type        string
}

type MiddlewareOption func(*MiddlewareInfo)

type MiddlewareInfo struct {
	Description    string
	SecurityScheme *SecuritySchemeDoc
	Headers        []HeaderDoc
}

type RouteOption func(*Route)

func NewRoute(method string, handler any, opts ...RouteOption) Route {
	route := Route{
		Method:  method,
		Path:    "",
		Handler: handler,
		Version: 1,
	}
	for _, opt := range opts {
		opt(&route)
	}
	return route
}

func NewGroup(path string, subRoutes []Route, opts ...RouteOption) Route {
	r := Route{
		Path:      path,
		SubRoutes: subRoutes,
	}
	for _, opt := range opts {
		opt(&r)
	}
	return r
}

type PathParamType int

const (
	ParamString PathParamType = iota
	ParamInt
	ParamInt8
	ParamInt16
	ParamInt32
	ParamInt64
	ParamUint
	ParamUint8
	ParamUint16
	ParamUint32
	ParamUint64
	ParamFloat32
	ParamFloat64
	ParamBool
)

func (t PathParamType) String() string {
	switch t {
	case ParamInt:
		return "int"
	case ParamInt8:
		return "int8"
	case ParamInt16:
		return "int16"
	case ParamInt32:
		return "int32"
	case ParamInt64:
		return "int64"
	case ParamUint:
		return "uint"
	case ParamUint8:
		return "uint8"
	case ParamUint16:
		return "uint16"
	case ParamUint32:
		return "uint32"
	case ParamUint64:
		return "uint64"
	case ParamFloat32:
		return "float32"
	case ParamFloat64:
		return "float64"
	case ParamBool:
		return "bool"
	default:
		return "string"
	}
}

func parsePathParamType(s string) PathParamType {
	switch s {
	case "int":
		return ParamInt
	case "int8":
		return ParamInt8
	case "int16":
		return ParamInt16
	case "int32":
		return ParamInt32
	case "int64":
		return ParamInt64
	case "uint":
		return ParamUint
	case "uint8":
		return ParamUint8
	case "uint16":
		return ParamUint16
	case "uint32":
		return ParamUint32
	case "uint64":
		return ParamUint64
	case "float32":
		return ParamFloat32
	case "float64":
		return ParamFloat64
	case "bool":
		return ParamBool
	default:
		return ParamString
	}
}

func parsePathParamTypes(path string) []PathParamType {
	var types []PathParamType
	start := -1
	for i := 0; i < len(path); i++ {
		if path[i] == '{' {
			start = i + 1
		}
		if path[i] == '}' && start != -1 {
			param := path[start:i]
			if idx := strings.IndexByte(param, ':'); idx != -1 {
				types = append(types, parsePathParamType(param[idx+1:]))
			} else {
				types = append(types, ParamString)
			}
			start = -1
		}
	}
	return types
}

func WithPath(path string) RouteOption {
	return func(r *Route) {
		r.Path = path
		r.PathParamTypes = parsePathParamTypes(path)
	}
}

func WithVersion(version int) RouteOption {
	return func(r *Route) {
		r.Version = version
	}
}

func WithMiddleware(handlers ...func(http.Handler) http.Handler) RouteOption {
	return func(r *Route) {
		r.Middlewares = &handlers
	}
}

func WithSummary(s string) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.Summary = s
	}
}

func WithDescription(s string) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.Description = s
	}
}

func WithTags(tags ...string) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.Tags = append(r.Doc.Tags, tags...)
	}
}

func WithRequest(req any) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.RequestType = req
	}
}

// WithQuery sets the query parameter type for auto-binding and OpenAPI docs.
// For GET endpoints, phastos binds URL query params into this type.
func WithQuery(query any) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.QueryType = query
	}
}

func WithResponse(resp any) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.ResponseType = resp
	}
}

func WithErrorResponse(status int, code string, desc string) RouteOption {
	return func(r *Route) {
		if r.Doc == nil {
			r.Doc = &RouteDoc{}
		}
		r.Doc.ErrorResponses = append(r.Doc.ErrorResponses, ErrorResponseDoc{
			StatusCode:  status,
			Code:        code,
			Description: desc,
		})
	}
}

func WithSecurity(schemeType, name, in string) MiddlewareOption {
	return func(m *MiddlewareInfo) {
		m.SecurityScheme = &SecuritySchemeDoc{
			Type: schemeType,
			Name: name,
			In:   in,
		}
	}
}

func WithRequiredHeader(name, description string, required bool) MiddlewareOption {
	return func(m *MiddlewareInfo) {
		m.Headers = append(m.Headers, HeaderDoc{
			Name:        name,
			Description: description,
			Required:    required,
			Type:        "string", //nolint:goconst
		})
	}
}

func WithMiddlewareDescription(desc string) MiddlewareOption {
	return func(m *MiddlewareInfo) {
		m.Description = desc
	}
}

func (r *Route) GetVersionedPath(controllerPath string) string {
	versionPrefix := "/v" + strconv.Itoa(r.Version)
	return versionPrefix + controllerPath + r.Path
}

type ControllerConfig struct {
	Path        string
	Routes      []Route
	Middlewares *[]func(http.Handler) http.Handler
}

type Controller interface {
	GetConfig() ControllerConfig
}

type Controllers interface {
	Register() []Controller
}

func (app *App) initRequest(r *http.Request) *Request {
	req := requestPool.Get().(*Request) //nolint:errcheck
	req.GetParams = func(key string, defaultValue ...string) string {
		var paramValue string
		if paramValue = chi.URLParam(r, key); paramValue == "" {
			for _, v := range defaultValue {
				paramValue = v
			}
		}
		return paramValue
	}
	req.GetFile = func(key string) (multipart.File, *multipart.FileHeader, error) {
		return r.FormFile(key)
	}
	req.GetQuery = func(i interface{}) error {
		if err := decoder.Decode(i, r.URL.Query()); err != nil {
			return BadRequest(err.Error(), "ERROR_PARSING_QUERY_PARAMS")
		}
		return app.requestValidator(i)
	}
	req.GetHeaders = func(i interface{}) error {
		if err := decoder.Decode(i, r.Header); err != nil {
			return BadRequest(err.Error(), "ERROR_PARSING_HEADER")
		}
		return app.requestValidator(i)
	}
	req.GetBody = func(i interface{}) error {
		contentType := filterFlags(r.Header.Get("Content-Type"))
		switch contentType {
		case ContentJSON:
			if err := getBodyFromJSON(r, i); err != nil {
				return BadRequest(err.Error(), ErrParsedBodyCode)
			}
		case ContentURLEncoded:
			if err := parseFormRequest(r); err != nil {
				return BadRequest(err.Error(), ErrParsedBodyCode)
			}
			if err := doHandleDecodeSchema(r, i); err != nil {
				return BadRequest(err.Error(), ErrDecodeBodyCode)
			}
		case ContentFormData:
			if err := parseMultiPartFormRequest(r, 32<<20); err != nil {
				return BadRequest(err.Error(), ErrParsedBodyCode)
			}
			if err := doHandleDecodeSchema(r, i); err != nil {
				return BadRequest(err.Error(), ErrDecodeBodyCode)
			}
		default:
			if err := getBodyFromJSON(r, i); err != nil {
				return BadRequest(err.Error(), ErrParsedBodyCode)
			}
		}
		return app.requestValidator(i)
	}
	return req
}

// ReleaseRequest resets and returns a Request to the pool.
func ReleaseRequest(req *Request) {
	*req = Request{} // zero all fields
	requestPool.Put(req)
}

type ControllerImpl struct {
	registeredMiddlewares map[string]any
	usedMiddlewareKeys    []string
}

func NewControllerImpl() *ControllerImpl {
	return &ControllerImpl{
		registeredMiddlewares: make(map[string]any),
	}
}

// SetRegisteredMiddlewares is called by App.AddController to inject the app-level middleware registry.
func (ctrl *ControllerImpl) SetRegisteredMiddlewares(m map[string]any) {
	ctrl.registeredMiddlewares = m
}

// UseMiddleware retrieves a registered middleware by key.
// Returns nil if the key is not found — caller should handle nil check or skip.
func (ctrl *ControllerImpl) UseMiddleware(key string) func(http.Handler) http.Handler {
	ctrl.usedMiddlewareKeys = append(ctrl.usedMiddlewareKeys, key)
	if ctrl.registeredMiddlewares == nil {
		return nil
	}
	if mw, ok := ctrl.registeredMiddlewares[key]; ok {
		if fn, ok := mw.(func(http.Handler) http.Handler); ok {
			return fn
		}
	}
	return nil
}

func (ctrl *ControllerImpl) GetUsedMiddlewareKeys() []string {
	return ctrl.usedMiddlewareKeys
}

func (ctrl *ControllerImpl) JoinMiddleware(handlers ...func(http.Handler) http.Handler) *[]func(http.Handler) http.Handler {
	return &handlers
}
