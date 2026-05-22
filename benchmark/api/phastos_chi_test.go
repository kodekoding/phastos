package benchmark

import (
	"context"
	"net/http"
	"testing"

	api "github.com/kodekoding/phastos/v2/go/api"
)

// --- JSON Response ---

type phastosChiJSONController struct {
	api.ControllerImpl
}

func (c *phastosChiJSONController) GetConfig() api.ControllerConfig {
	return api.ControllerConfig{
		Path: "/json",
		Routes: []api.Route{
			api.NewRoute("GET", func(request api.Request, ctx context.Context) *api.Response {
				return api.NewResponse().SetMessage("hello")
			}, api.WithPath("")),
		},
	}
}

// --- Path Parameter ---

type phastosChiPathParamController struct {
	api.ControllerImpl
}

func (c *phastosChiPathParamController) GetConfig() api.ControllerConfig {
	return api.ControllerConfig{
		Path: "/path",
		Routes: []api.Route{
			api.NewRoute("GET", func(request api.Request, ctx context.Context) *api.Response {
				id := request.GetParams("id")
				return api.NewResponse().SetData(map[string]string{"id": id})
			}, api.WithPath("/{id}")),
		},
	}
}

// --- Query Parameter ---

type phastosChiQueryParamController struct {
	api.ControllerImpl
}

func (c *phastosChiQueryParamController) GetConfig() api.ControllerConfig {
	return api.ControllerConfig{
		Path: "/query",
		Routes: []api.Route{
			api.NewRoute("GET", func(request api.Request, ctx context.Context) *api.Response {
				var req struct {
					Name string `schema:"name" validate:"required"`
				}
				if err := request.GetQuery(&req); err != nil {
					return api.NewResponse().SetError(err)
				}
				return api.NewResponse().SetData(map[string]string{"name": req.Name})
			}, api.WithPath("")),
		},
	}
}

// --- Middleware Chain ---

type phastosChiMiddlewareController struct {
	api.ControllerImpl
}

func (c *phastosChiMiddlewareController) GetConfig() api.ControllerConfig {
	return api.ControllerConfig{
		Path: "/middleware",
		Middlewares: &[]func(http.Handler) http.Handler{
			dummyMiddleware1,
			dummyMiddleware2,
			dummyMiddleware3,
		},
		Routes: []api.Route{
			api.NewRoute("GET", func(request api.Request, ctx context.Context) *api.Response {
				return api.NewResponse().SetMessage("hello")
			}, api.WithPath("")),
		},
	}
}

// dummy middlewares for benchmarking overhead
// These set headers on the ResponseWriter (not the Request) to avoid
// expensive request header copy-on-write mutations.
func dummyMiddleware1(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Mw1", "true")
		next.ServeHTTP(w, r)
	})
}

func dummyMiddleware2(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Mw2", "true")
		next.ServeHTTP(w, r)
	})
}

func dummyMiddleware3(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Mw3", "true")
		next.ServeHTTP(w, r)
	})
}

// setupPhastosChiApp creates a Phastos app and registers controllers.
// It uses AddController which internally calls flushPendingMiddlewares.
// The handler is wrapped with InitHandler to inject request-id into context
// (required by Phastos' requestLogger middleware).
func setupPhastosChiApp(controllers ...api.Controller) http.Handler {
	app := api.NewApp(
		api.WithAppPort(0),
		api.WithPprof(false),
		api.WithAPITimeout(0), // sync handler path: skip goroutine+channel+timeout
	)
	app.Init()

	for _, ctrl := range controllers {
		app.AddController(ctrl)
	}

	return api.InitHandler(app.Http)
}

func BenchmarkPhastosChi_JSON(b *testing.B) {
	handler := setupPhastosChiApp(&phastosChiJSONController{})
	runNetHTTPBenchmark(b, handler, "GET", "/v1/json")
}

func BenchmarkPhastosChi_PathParam(b *testing.B) {
	handler := setupPhastosChiApp(&phastosChiPathParamController{})
	runNetHTTPBenchmark(b, handler, "GET", "/v1/path/42")
}

func BenchmarkPhastosChi_QueryParam(b *testing.B) {
	handler := setupPhastosChiApp(&phastosChiQueryParamController{})
	runNetHTTPBenchmark(b, handler, "GET", "/v1/query?name=hello")
}

func BenchmarkPhastosChi_Middleware(b *testing.B) {
	handler := setupPhastosChiApp(&phastosChiMiddlewareController{})
	runNetHTTPBenchmark(b, handler, "GET", "/v1/middleware")
}
