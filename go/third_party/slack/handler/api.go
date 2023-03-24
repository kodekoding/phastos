package handler

import (
	"context"
	"net/http"
	"strconv"
)

type (
	APIRequest struct {
		GetParams  func(key string, defaultValue ...string) string
		GetQuery   func(interface{}) error
		GetBody    func(interface{}) error
		GetHeaders func(interface{}) error
	}

	APIHandler func(ctx context.Context, request APIRequest) error

	RouteOption func(event *Route)
	Route       struct {
		Method      string
		Path        string
		Handler     APIHandler
		Version     int
		Middlewares *[]func(http.Handler) http.Handler
	}
	ControllerConfig struct {
		Handler []Route
	}

	Controller interface {
		GetConfig() ControllerConfig
	}

	ControllerImpl struct{}
)

func RegisterRoute(method string, handler APIHandler, opts ...RouteOption) Route {
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

func (ctrl *ControllerImpl) JoinMiddleware(handlers ...func(http.Handler) http.Handler) *[]func(http.Handler) http.Handler {
	return &handlers
}
