package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRoute(t *testing.T) {
	dummyHandler := func(req Request, ctx context.Context) *Response {
		return NewResponse().SetMessage("ok")
	}

	t.Run("should create route with defaults", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler)
		assert.Equal(t, http.MethodGet, route.Method)
		assert.Equal(t, "", route.Path)
		assert.Equal(t, 1, route.Version)
		assert.Nil(t, route.Middlewares)
	})

	t.Run("should apply WithPath option", func(t *testing.T) {
		route := NewRoute(http.MethodPost, dummyHandler, WithPath("/users"))
		assert.Equal(t, http.MethodPost, route.Method)
		assert.Equal(t, "/users", route.Path)
	})

	t.Run("should apply WithVersion option", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler, WithVersion(2))
		assert.Equal(t, 2, route.Version)
	})

	t.Run("should apply WithMiddleware option", func(t *testing.T) {
		mw := func(next http.Handler) http.Handler { return next }
		route := NewRoute(http.MethodGet, dummyHandler, WithMiddleware(mw))
		assert.NotNil(t, route.Middlewares)
		assert.Len(t, *route.Middlewares, 1)
	})

	t.Run("should apply multiple options", func(t *testing.T) {
		route := NewRoute(http.MethodPut, dummyHandler,
			WithPath("/accounts/{id}"),
			WithVersion(3),
		)
		assert.Equal(t, http.MethodPut, route.Method)
		assert.Equal(t, "/accounts/{id}", route.Path)
		assert.Equal(t, 3, route.Version)
	})
}

func TestRoute_GetVersionedPath(t *testing.T) {
	dummyHandler := func(req Request, ctx context.Context) *Response { return nil }

	t.Run("should generate versioned path v1", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler, WithPath("/list"))
		result := route.GetVersionedPath("/users")
		assert.Equal(t, "/v1/users/list", result)
	})

	t.Run("should generate versioned path v2", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler, WithPath("/detail"), WithVersion(2))
		result := route.GetVersionedPath("/accounts")
		assert.Equal(t, "/v2/accounts/detail", result)
	})

	t.Run("should handle empty route path", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler)
		result := route.GetVersionedPath("/users")
		assert.Equal(t, "/v1/users", result)
	})

	t.Run("should handle empty controller path", func(t *testing.T) {
		route := NewRoute(http.MethodGet, dummyHandler, WithPath("/list"))
		result := route.GetVersionedPath("")
		assert.Equal(t, "/v1/list", result)
	})
}

func TestControllerImpl_UseMiddleware(t *testing.T) {
	t.Run("should return nil when no middlewares registered", func(t *testing.T) {
		ctrl := &ControllerImpl{}
		mw := ctrl.UseMiddleware("auth")
		assert.Nil(t, mw)
	})

	t.Run("should return nil for unknown key", func(t *testing.T) {
		ctrl := &ControllerImpl{
			registeredMiddlewares: map[string]any{},
		}
		mw := ctrl.UseMiddleware("nonexistent")
		assert.Nil(t, mw)
	})

	t.Run("should return middleware for valid key", func(t *testing.T) {
		expectedMw := func(next http.Handler) http.Handler { return next }
		ctrl := &ControllerImpl{
			registeredMiddlewares: map[string]any{
				"auth": expectedMw,
			},
		}
		mw := ctrl.UseMiddleware("auth")
		assert.NotNil(t, mw)
	})

	t.Run("should return nil when value is not a middleware func", func(t *testing.T) {
		ctrl := &ControllerImpl{
			registeredMiddlewares: map[string]any{
				"auth": "not-a-function",
			},
		}
		mw := ctrl.UseMiddleware("auth")
		assert.Nil(t, mw)
	})
}

func TestControllerImpl_JoinMiddleware(t *testing.T) {
	ctrl := &ControllerImpl{}
	mw1 := func(next http.Handler) http.Handler { return next }
	mw2 := func(next http.Handler) http.Handler { return next }

	result := ctrl.JoinMiddleware(mw1, mw2)
	assert.NotNil(t, result)
	assert.Len(t, *result, 2)
}
