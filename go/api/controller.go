package api

import (
	"context"
	"mime/multipart"
	"net/http"
	"strconv"
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
	Method      string
	Path        string
	Handler     Handler
	Version     int
	Middlewares *[]func(http.Handler) http.Handler
}

type RouteOption func(*Route)

func NewRoute(method string, handler Handler, opts ...RouteOption) Route {
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

func WithPath(path string) RouteOption {
	return func(r *Route) {
		r.Path = path
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

func (ctrl *ControllerImpl) JoinMiddleware(handlers ...func(http.Handler) http.Handler) *[]func(http.Handler) http.Handler {
	return &handlers
}
