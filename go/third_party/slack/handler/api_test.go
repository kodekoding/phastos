package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterRoute(t *testing.T) {
	route := RegisterRoute(http.MethodGet, func(ctx context.Context, req APIRequest) error {
		return nil
	})
	assert.Equal(t, http.MethodGet, route.Method)
	assert.Equal(t, "", route.Path)
	assert.Equal(t, 1, route.Version)
}

func TestRegisterRouteWithPath(t *testing.T) {
	route := RegisterRoute(http.MethodPost, func(ctx context.Context, req APIRequest) error {
		return nil
	}, WithPath("/users"))
	assert.Equal(t, "/users", route.Path)
}

func TestRegisterRouteWithVersion(t *testing.T) {
	route := RegisterRoute(http.MethodGet, func(ctx context.Context, req APIRequest) error {
		return nil
	}, WithVersion(2))
	assert.Equal(t, 2, route.Version)
}

func TestRegisterRouteWithMiddleware(t *testing.T) {
	mw := func(next http.Handler) http.Handler {
		return next
	}
	route := RegisterRoute(http.MethodGet, func(ctx context.Context, req APIRequest) error {
		return nil
	}, WithMiddleware(mw))
	assert.NotNil(t, route.Middlewares)
}

func TestRouteGetVersionedPath(t *testing.T) {
	route := Route{
		Method:  http.MethodGet,
		Path:    "/users",
		Version: 1,
	}
	path := route.GetVersionedPath("/api")
	assert.Equal(t, "/v1/api/users", path)
}

func TestRouteGetVersionedPathV2(t *testing.T) {
	route := Route{
		Method:  http.MethodGet,
		Path:    "/items",
		Version: 2,
	}
	path := route.GetVersionedPath("/api")
	assert.Equal(t, "/v2/api/items", path)
}

func TestControllerConfig(t *testing.T) {
	cfg := ControllerConfig{
		Handler: []Route{
			RegisterRoute(http.MethodGet, func(ctx context.Context, req APIRequest) error { return nil }),
		},
	}
	assert.Len(t, cfg.Handler, 1)
}

func TestControllerImpl(t *testing.T) {
	ctrl := &ControllerImpl{}
	assert.NotNil(t, ctrl)
}

func TestControllerImplJoinMiddleware(t *testing.T) {
	ctrl := &ControllerImpl{}
	mw1 := func(next http.Handler) http.Handler { return next }
	mw2 := func(next http.Handler) http.Handler { return next }
	result := ctrl.JoinMiddleware(mw1, mw2)
	assert.NotNil(t, result)
	assert.Len(t, *result, 2)
}

func TestAPIRequestStruct(t *testing.T) {
	req := APIRequest{
		GetParams: func(key string, defaultValue ...string) string { return "val" },
	}
	assert.NotNil(t, req.GetParams)
}

func TestRouteStruct(t *testing.T) {
	route := Route{
		Method:  http.MethodPost,
		Path:    "/test",
		Version: 3,
	}
	assert.Equal(t, http.MethodPost, route.Method)
	assert.Equal(t, "/test", route.Path)
	assert.Equal(t, 3, route.Version)
}
