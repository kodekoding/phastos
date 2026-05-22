package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// --- FastRequest ---

func TestFastRequest_GetParam(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.SetUserValue("id", "123")
	ctx.SetUserValue("name", "test")

	app := &FastHttpApp{}
	req := &FastRequest{ctx: ctx, app: app}

	t.Run("returns value for existing param", func(t *testing.T) {
		assert.Equal(t, "123", req.GetParam("id"))
	})

	t.Run("returns empty string for missing param", func(t *testing.T) {
		assert.Equal(t, "", req.GetParam("missing"))
	})

	t.Run("returns default value for missing param", func(t *testing.T) {
		assert.Equal(t, "default", req.GetParam("missing", "default"))
	})

	t.Run("returns empty string when UserValue is not a string", func(t *testing.T) {
		ctx.SetUserValue("num", 42)
		assert.Equal(t, "", req.GetParam("num"))
	})
}

func TestFastRequest_GetQuery(t *testing.T) {
	app := &FastHttpApp{}

	t.Run("decodes query params into struct", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.QueryArgs().Set("name", "John")
		ctx.QueryArgs().Set("email", "john@example.com")

		req := &FastRequest{ctx: ctx, app: app}

		type queryStruct struct {
			Name  string `schema:"name" validate:"required"`
			Email string `schema:"email" validate:"required"`
		}
		var dest queryStruct
		err := req.GetQuery(&dest)
		require.NoError(t, err)
		assert.Equal(t, "John", dest.Name)
		assert.Equal(t, "john@example.com", dest.Email)
	})

	t.Run("returns error for invalid query params", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		// No params set, but required validation will fail
		req := &FastRequest{ctx: ctx, app: app}

		type queryStruct struct {
			Name string `schema:"name" validate:"required"`
		}
		var dest queryStruct
		err := req.GetQuery(&dest)
		assert.NotNil(t, err)
	})
}

func TestFastRequest_GetHeaders(t *testing.T) {
	app := &FastHttpApp{}

	t.Run("decodes headers into struct", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.Set("X-Custom-Header", "custom-value")

		req := &FastRequest{ctx: ctx, app: app}

		type headerStruct struct {
			CustomHeader string `schema:"X-Custom-Header"`
		}
		var dest headerStruct
		err := req.GetHeaders(&dest)
		require.NoError(t, err)
		assert.Equal(t, "custom-value", dest.CustomHeader)
	})
}

func TestFastRequest_GetBody(t *testing.T) {
	app := &FastHttpApp{}

	t.Run("parses JSON body", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetContentType("application/json")
		ctx.Request.SetBody([]byte(`{"name":"John","email":"john@example.com"}`))

		req := &FastRequest{ctx: ctx, app: app}

		type bodyStruct struct {
			Name  string `json:"name" validate:"required"`
			Email string `json:"email" validate:"required"`
		}
		var dest bodyStruct
		err := req.GetBody(&dest)
		require.NoError(t, err)
		assert.Equal(t, "John", dest.Name)
		assert.Equal(t, "john@example.com", dest.Email)
	})

	t.Run("parses URL-encoded body", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetContentType("application/x-www-form-urlencoded")
		ctx.Request.SetBody([]byte("name=John&email=john%40example.com"))

		req := &FastRequest{ctx: ctx, app: app}

		type bodyStruct struct {
			Name  string `schema:"name" validate:"required"`
			Email string `schema:"email" validate:"required"`
		}
		var dest bodyStruct
		err := req.GetBody(&dest)
		require.NoError(t, err)
		assert.Equal(t, "John", dest.Name)
		assert.Equal(t, "john@example.com", dest.Email)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetContentType("application/json")
		ctx.Request.SetBody([]byte(`{invalid json`))

		req := &FastRequest{ctx: ctx, app: app}

		type bodyStruct struct {
			Name string `json:"name"`
		}
		var dest bodyStruct
		err := req.GetBody(&dest)
		assert.NotNil(t, err)
	})

	t.Run("returns validation error for missing required field", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.Header.SetContentType("application/json")
		ctx.Request.SetBody([]byte(`{"name":""}`))

		req := &FastRequest{ctx: ctx, app: app}

		type bodyStruct struct {
			Name string `json:"name" validate:"required"`
		}
		var dest bodyStruct
		err := req.GetBody(&dest)
		assert.NotNil(t, err)
	})
}

func TestFastRequest_Header(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("Authorization", "Bearer token123")
	ctx.Request.Header.Set("X-Custom", "value")

	app := &FastHttpApp{}
	req := &FastRequest{ctx: ctx, app: app}

	assert.Equal(t, "Bearer token123", req.Header("Authorization"))
	assert.Equal(t, "value", req.Header("X-Custom"))
	assert.Equal(t, "", req.Header("Missing"))
}

func TestFastRequest_Ctx(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	app := &FastHttpApp{}
	req := &FastRequest{ctx: ctx, app: app}

	assert.Equal(t, ctx, req.Ctx())
}

func TestFastRequest_release(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	app := &FastHttpApp{}
	req := &FastRequest{ctx: ctx, app: app}
	req.release()
	assert.Nil(t, req.ctx)
	assert.Nil(t, req.app)
}

// --- FastResponse ---

func TestNewFastResponse(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	fr := NewFastResponse(ctx)
	assert.NotNil(t, fr)
	assert.Equal(t, ctx, fr.ctx)
	assert.Equal(t, http.StatusOK, fr.statusCode)
}

func TestFastResponse_SetMessage(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	fr := NewFastResponse(ctx)
	result := fr.SetMessage("hello")
	assert.Equal(t, fr, result)
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))
	assert.Contains(t, string(ctx.Response.Body()), "hello")
	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
}

func TestFastResponse_SetData(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	fr := NewFastResponse(ctx)
	result := fr.SetData(map[string]string{"key": "value"})
	assert.Equal(t, fr, result)
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))

	var body map[string]string
	err := json.Unmarshal(ctx.Response.Body(), &body)
	require.NoError(t, err)
	assert.Equal(t, "value", body["key"])
}

func TestFastResponse_SetError(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	fr := NewFastResponse(ctx)
	err := Unauthorized("unauthorized", "UNAUTH")
	result := fr.SetError(err)
	assert.Equal(t, fr, result)
	assert.Equal(t, http.StatusUnauthorized, ctx.Response.StatusCode())
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))

	var body map[string]interface{}
	jsonErr := json.Unmarshal(ctx.Response.Body(), &body)
	require.NoError(t, jsonErr)
	assert.Equal(t, "unauthorized", body["message"])
	assert.Equal(t, "UNAUTH", body["code"])
}

func TestFastResponse_SetStatusCode(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	fr := NewFastResponse(ctx)
	result := fr.SetStatusCode(http.StatusCreated)
	assert.Equal(t, fr, result)
	assert.Equal(t, http.StatusCreated, fr.statusCode)

	// Now write a message to verify the status code is used
	fr.SetMessage("created")
	assert.Equal(t, http.StatusCreated, ctx.Response.StatusCode())
}

func TestFastResponse_release(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	fr := NewFastResponse(ctx)
	fr.release()
	assert.Nil(t, fr.ctx)
	assert.Equal(t, http.StatusOK, fr.statusCode)
}

// --- fastRouter ---

func TestNewFastRouter(t *testing.T) {
	r := newFastRouter()
	assert.NotNil(t, r)
	assert.NotNil(t, r.exactRoutes)
	assert.NotNil(t, r.notFound)
	assert.Empty(t, r.paramRoutes)
}

func TestFastRouter_Handle_ExactPath(t *testing.T) {
	r := newFastRouter()
	called := false
	handler := func(ctx *fasthttp.RequestCtx) {
		called = true
	}
	r.Handle("GET", "/v1/test", handler)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/v1/test")
	ctx.Request.Header.SetMethod("GET")

	r.ServeHTTP(ctx)
	assert.True(t, called)
}

func TestFastRouter_Handle_ParamPath(t *testing.T) {
	r := newFastRouter()
	var capturedID string
	handler := func(ctx *fasthttp.RequestCtx) {
		capturedID = string(ctx.UserValue("id").(string)) //nolint:errcheck
	}
	r.Handle("GET", "/v1/users/:id", handler)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/v1/users/42")
	ctx.Request.Header.SetMethod("GET")

	r.ServeHTTP(ctx)
	assert.Equal(t, "42", capturedID)
}

func TestFastRouter_Handle_ParamPathWithBraces(t *testing.T) {
	r := newFastRouter()
	var capturedID string
	handler := func(ctx *fasthttp.RequestCtx) {
		v := ctx.UserValue("id")
		if v != nil {
			capturedID = v.(string) //nolint:errcheck
		}
	}
	r.Handle("GET", "/v1/items/{id}", handler)

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/v1/items/99")
	ctx.Request.Header.SetMethod("GET")

	r.ServeHTTP(ctx)
	assert.Equal(t, "99", capturedID)
}

func TestFastRouter_ServeHTTP_NotFound(t *testing.T) {
	r := newFastRouter()
	r.Handle("GET", "/exists", func(ctx *fasthttp.RequestCtx) {})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/missing")
	ctx.Request.Header.SetMethod("GET")

	r.ServeHTTP(ctx)
	assert.Equal(t, http.StatusNotFound, ctx.Response.StatusCode())
	assert.Equal(t, "application/json", string(ctx.Response.Header.ContentType()))
}

func TestFastRouter_ServeHTTP_MethodMismatch(t *testing.T) {
	r := newFastRouter()
	r.Handle("GET", "/v1/test", func(ctx *fasthttp.RequestCtx) {})

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/v1/test")
	ctx.Request.Header.SetMethod("POST")

	r.ServeHTTP(ctx)
	assert.Equal(t, http.StatusNotFound, ctx.Response.StatusCode())
}

func TestFastRouter_Handle_MultipleExactPaths(t *testing.T) {
	r := newFastRouter()
	var pathCalled string
	r.Handle("GET", "/v1/a", func(ctx *fasthttp.RequestCtx) { pathCalled = "a" })
	r.Handle("GET", "/v1/b", func(ctx *fasthttp.RequestCtx) { pathCalled = "b" })

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/v1/a")
	ctx.Request.Header.SetMethod("GET")
	r.ServeHTTP(ctx)
	assert.Equal(t, "a", pathCalled)

	ctx2 := &fasthttp.RequestCtx{}
	ctx2.Request.SetRequestURI("/v1/b")
	ctx2.Request.Header.SetMethod("GET")
	r.ServeHTTP(ctx2)
	assert.Equal(t, "b", pathCalled)
}

// --- matchParamPath ---

func TestMatchParamPath(t *testing.T) {
	t.Run("matches single param at end", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		result := matchParamPath("/v1/users/:id", "/v1/users/42", ctx)
		assert.True(t, result)
		assert.Equal(t, "42", string(ctx.UserValue("id").(string))) //nolint:errcheck
	})

	t.Run("matches multiple params", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		result := matchParamPath("/v1/:org/:repo", "/v1/github/api", ctx)
		assert.True(t, result)
		assert.Equal(t, "github", string(ctx.UserValue("org").(string)))  //nolint:errcheck
		assert.Equal(t, "api", string(ctx.UserValue("repo").(string)))    //nolint:errcheck
	})

	t.Run("matches brace-style params", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		result := matchParamPath("/v1/items/{id}", "/v1/items/99", ctx)
		assert.True(t, result)
		assert.Equal(t, "99", string(ctx.UserValue("id").(string))) //nolint:errcheck
	})

	t.Run("rejects non-matching path", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		result := matchParamPath("/v1/users/:id", "/v1/items/42", ctx)
		assert.False(t, result)
	})

	t.Run("rejects different segment count", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		result := matchParamPath("/v1/users/:id", "/v1/users", ctx)
		assert.False(t, result)
	})

	t.Run("rejects empty path vs non-empty pattern", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		result := matchParamPath("/v1/users/:id", "", ctx)
		assert.False(t, result)
	})

	t.Run("matches root path", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		result := matchParamPath("/", "/", ctx)
		assert.True(t, result)
	})

	t.Run("matches literal segments before param", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		result := matchParamPath("/api/v1/users/:id", "/api/v1/users/5", ctx)
		assert.True(t, result)
		assert.Equal(t, "5", string(ctx.UserValue("id").(string))) //nolint:errcheck
	})

	t.Run("rejects wrong literal segment", func(t *testing.T) {
		ctx := &fasthttp.RequestCtx{}
		result := matchParamPath("/api/v1/users/:id", "/api/v2/users/5", ctx)
		assert.False(t, result)
	})
}

// --- FastRoute.GetVersionedPath ---

func TestFastRoute_GetVersionedPath(t *testing.T) {
	t.Run("default version 1", func(t *testing.T) {
		route := FastRoute{Path: "/list", Version: 1}
		result := route.GetVersionedPath("/users")
		assert.Equal(t, "/v1/users/list", result)
	})

	t.Run("custom version", func(t *testing.T) {
		route := FastRoute{Path: "/detail", Version: 2}
		result := route.GetVersionedPath("/accounts")
		assert.Equal(t, "/v2/accounts/detail", result)
	})

	t.Run("empty route path", func(t *testing.T) {
		route := FastRoute{Version: 1}
		result := route.GetVersionedPath("/users")
		assert.Equal(t, "/v1/users", result)
	})

	t.Run("empty controller path", func(t *testing.T) {
		route := FastRoute{Path: "/list", Version: 1}
		result := route.GetVersionedPath("")
		assert.Equal(t, "/v1/list", result)
	})
}

// --- FastHttpApp ---

func TestNewFastHttpApp(t *testing.T) {
	app := NewFastHttpApp()
	assert.NotNil(t, app)
	assert.NotNil(t, app.middlewares)
	assert.False(t, app.pprofEnabled)
	assert.Equal(t, 0, app.apiTimeout)
	assert.True(t, app.syncMode)
}

func TestFastHttpApp_Init(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()
	assert.NotNil(t, app.router)
	assert.True(t, app.pendingMiddlewares)
}

func TestFastHttpApp_RegisterMiddlewareFunc(t *testing.T) {
	app := NewFastHttpApp()
	mw := func(next fasthttp.RequestHandler) fasthttp.RequestHandler { return next }
	app.RegisterMiddlewareFunc("auth", mw)
	assert.NotNil(t, app.middlewares["auth"])
}

func TestFastHttpApp_AddGlobalMiddleware(t *testing.T) {
	app := NewFastHttpApp()
	mw := func(next fasthttp.RequestHandler) fasthttp.RequestHandler { return next }
	app.AddGlobalMiddleware(mw)
	assert.Len(t, app.globalMiddlewares, 1)
	assert.True(t, app.pendingMiddlewares)
}

func TestFastHttpApp_AddController(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	handler := func(req FastRequest, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}
	routes := []FastRoute{
		{Method: "GET", Path: "/list", Handler: handler, Version: 1},
	}

	ctrl := &testFastController{
		config: FastControllerConfig{
			Path:   "/users",
			Routes: routes,
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 2, app.TotalEndpoints) // 1 route + 1 (/ping)
}

func TestFastHttpApp_AddController_WithDirectHandler(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	directHandler := func(req *FastRequest) *FastResponse {
		return NewFastResponse(req.Ctx()).SetMessage("direct")
	}
	routes := []FastRoute{
		{Method: "GET", Path: "/direct", DirectHandler: directHandler, Version: 1},
	}

	ctrl := &testFastController{
		config: FastControllerConfig{
			Path:   "/items",
			Routes: routes,
		},
	}

	app.AddController(ctrl)
	assert.Equal(t, 2, app.TotalEndpoints) // 1 route + 1 (/ping)
}

func TestFastHttpApp_AddController_WithMiddlewares(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	handler := func(req FastRequest, ctx context.Context) *Response {
		return NewResponse().SetMessage("hello")
	}

	mwCalled := false
	routeMw := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			mwCalled = true
			next(ctx)
		}
	}

	ctrlMwCalled := false
	ctrlMw := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			ctrlMwCalled = true
			next(ctx)
		}
	}

	routes := []FastRoute{
		{Method: "GET", Path: "/list", Handler: handler, Version: 1, Middlewares: []FastMiddleware{routeMw}},
	}

	ctrl := &testFastController{
		config: FastControllerConfig{
			Path:        "/users",
			Routes:      routes,
			Middlewares: []FastMiddleware{ctrlMw},
		},
	}

	app.AddController(ctrl)

	// Test that route is reachable via the router
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/v1/users/list")
	ctx.Request.Header.SetMethod("GET")

	handler2 := app.Handler()
	handler2(ctx)

	assert.True(t, mwCalled, "route middleware should be called")
	assert.True(t, ctrlMwCalled, "controller middleware should be called")
}

func TestFastHttpApp_PingEndpoint(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	// Get the handler chain
	handler := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/ping")
	ctx.Request.Header.SetMethod("GET")

	handler(ctx)

	assert.Equal(t, http.StatusOK, ctx.Response.StatusCode())
	body := string(ctx.Response.Body())
	assert.Contains(t, body, "pong")
}

func TestFastHttpApp_NotFoundRoute(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()

	handler := app.Handler()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/nonexistent")
	ctx.Request.Header.SetMethod("GET")

	handler(ctx)

	assert.Equal(t, http.StatusNotFound, ctx.Response.StatusCode())
}

func TestFastHttpApp_flushPendingMiddlewares(t *testing.T) {
	app := NewFastHttpApp()
	app.Init()
	app.flushPendingMiddlewares() // first flush
	app.flushPendingMiddlewares() // second should be no-op
}

func TestFastHttpApp_requestValidator(t *testing.T) {
	app := NewFastHttpApp()

	t.Run("valid struct returns nil", func(t *testing.T) {
		type validStruct struct {
			Name string `validate:"required"`
		}
		err := app.requestValidator(validStruct{Name: "John"})
		assert.Nil(t, err)
	})

	t.Run("invalid struct returns HttpError", func(t *testing.T) {
		type invalidStruct struct {
			Name string `validate:"required"`
		}
		err := app.requestValidator(invalidStruct{Name: ""})
		assert.NotNil(t, err)
		httpErr, ok := err.(*HttpError)
		require.True(t, ok)
		assert.Equal(t, http.StatusUnprocessableEntity, httpErr.Status)
		assert.Equal(t, "VALIDATION_ERROR", httpErr.Code)
	})
}

// testFastController is a test implementation of FastController.
type testFastController struct {
	config FastControllerConfig
}

func (c *testFastController) GetConfig() FastControllerConfig {
	return c.config
}
