package api

import (
	"context"
	"mime/multipart"
	"net/http"
	"strconv"
)

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

type ControllerImpl struct {
}

func (ctrl *ControllerImpl) JoinMiddleware(handlers ...func(http.Handler) http.Handler) *[]func(http.Handler) http.Handler {
	return &handlers
}
