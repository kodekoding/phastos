package benchmark

import (
	"context"
	"testing"

	api "github.com/kodekoding/phastos/v2/go/api"
	"github.com/valyala/fasthttp"
)

// --- Phastos FastHttp (native) — compatibility path using FastHandler ---

type fastJSONController struct{}

func (c *fastJSONController) GetConfig() api.FastControllerConfig {
	return api.FastControllerConfig{
		Path: "/json",
		Routes: []api.FastRoute{
			{
				Method:  "GET",
				Handler: func(req api.FastRequest, ctx context.Context) *api.Response {
					return api.NewResponse().SetMessage("hello")
				},
			},
		},
	}
}

type fastPathParamController struct{}

func (c *fastPathParamController) GetConfig() api.FastControllerConfig {
	return api.FastControllerConfig{
		Path: "/path",
		Routes: []api.FastRoute{
			{
				Method:  "GET",
				Path:    "/:id",
				Handler: func(req api.FastRequest, ctx context.Context) *api.Response {
					id := req.GetParam("id")
					return api.NewResponse().SetData(map[string]string{"id": id})
				},
			},
		},
	}
}

type fastQueryParamController struct{}

func (c *fastQueryParamController) GetConfig() api.FastControllerConfig {
	return api.FastControllerConfig{
		Path: "/query",
		Routes: []api.FastRoute{
			{
				Method:  "GET",
				Handler: func(req api.FastRequest, ctx context.Context) *api.Response {
					var q struct {
						Name string `schema:"name" validate:"required"`
					}
					if err := req.GetQuery(&q); err != nil {
						return api.NewResponse().SetError(err)
					}
					return api.NewResponse().SetData(map[string]string{"name": q.Name})
				},
			},
		},
	}
}

func fastDummyMiddleware1(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.Set("X-Mw1", "true")
		next(ctx)
	}
}

func fastDummyMiddleware2(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.Set("X-Mw2", "true")
		next(ctx)
	}
}

func fastDummyMiddleware3(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.Set("X-Mw3", "true")
		next(ctx)
	}
}

type fastMiddlewareController struct{}

func (c *fastMiddlewareController) GetConfig() api.FastControllerConfig {
	return api.FastControllerConfig{
		Path: "/middleware",
		Middlewares: []api.FastMiddleware{
			fastDummyMiddleware1,
			fastDummyMiddleware2,
			fastDummyMiddleware3,
		},
		Routes: []api.FastRoute{
			{
				Method:  "GET",
				Handler: func(req api.FastRequest, ctx context.Context) *api.Response {
					return api.NewResponse().SetMessage("hello")
				},
			},
		},
	}
}

func setupPhastosFastHttpNativeApp(controllers ...api.FastController) fasthttp.RequestHandler {
	app := api.NewFastHttpApp()
	app.Init()

	for _, ctrl := range controllers {
		app.AddController(ctrl)
	}

	return app.Handler()
}

func BenchmarkPhastosFastHttpNative_JSON(b *testing.B) {
	handler := setupPhastosFastHttpDirectApp(&directJSONController{})
	runFastHTTPBenchmark(b, handler, "GET", "/v1/json")
}

func BenchmarkPhastosFastHttpNative_PathParam(b *testing.B) {
	handler := setupPhastosFastHttpDirectApp(&directPathParamController{})
	runFastHTTPBenchmark(b, handler, "GET", "/v1/path/42")
}

func BenchmarkPhastosFastHttpNative_QueryParam(b *testing.B) {
	handler := setupPhastosFastHttpNativeApp(&fastQueryParamController{})
	runFastHTTPBenchmark(b, handler, "GET", "/v1/query?name=hello")
}

func BenchmarkPhastosFastHttpNative_Middleware(b *testing.B) {
	handler := setupPhastosFastHttpDirectApp(&directMiddlewareController{})
	runFastHTTPBenchmark(b, handler, "GET", "/v1/middleware")
}

// --- Phastos FastHttp (direct) — optimized path using FastDirectHandler + FastResponse ---
// This bypasses Response + json.Marshal entirely for simple responses.

type directJSONController struct{}

func (c *directJSONController) GetConfig() api.FastControllerConfig {
	return api.FastControllerConfig{
		Path: "/json",
		Routes: []api.FastRoute{
			{
				Method: "GET",
				DirectHandler: func(req *api.FastRequest) *api.FastResponse {
					return api.NewFastResponse(req.Ctx()).SetMessage("hello")
				},
			},
		},
	}
}

type directPathParamController struct{}

func (c *directPathParamController) GetConfig() api.FastControllerConfig {
	return api.FastControllerConfig{
		Path: "/path",
		Routes: []api.FastRoute{
			{
				Method: "GET",
				Path:   "/:id",
				DirectHandler: func(req *api.FastRequest) *api.FastResponse {
					id := req.GetParam("id")
					return api.NewFastResponse(req.Ctx()).SetData(map[string]string{"id": id})
				},
			},
		},
	}
}

type directMiddlewareController struct{}

func (c *directMiddlewareController) GetConfig() api.FastControllerConfig {
	return api.FastControllerConfig{
		Path: "/middleware",
		Middlewares: []api.FastMiddleware{
			fastDummyMiddleware1,
			fastDummyMiddleware2,
			fastDummyMiddleware3,
		},
		Routes: []api.FastRoute{
			{
				Method: "GET",
				DirectHandler: func(req *api.FastRequest) *api.FastResponse {
					return api.NewFastResponse(req.Ctx()).SetMessage("hello")
				},
			},
		},
	}
}

func setupPhastosFastHttpDirectApp(controllers ...api.FastController) fasthttp.RequestHandler {
	app := api.NewFastHttpApp()
	app.Init()

	for _, ctrl := range controllers {
		app.AddController(ctrl)
	}

	return app.Handler()
}

func BenchmarkPhastosFastHttpDirect_JSON(b *testing.B) {
	handler := setupPhastosFastHttpDirectApp(&directJSONController{})
	runFastHTTPBenchmark(b, handler, "GET", "/v1/json")
}

func BenchmarkPhastosFastHttpDirect_PathParam(b *testing.B) {
	handler := setupPhastosFastHttpDirectApp(&directPathParamController{})
	runFastHTTPBenchmark(b, handler, "GET", "/v1/path/42")
}

func BenchmarkPhastosFastHttpDirect_Middleware(b *testing.B) {
	handler := setupPhastosFastHttpDirectApp(&directMiddlewareController{})
	runFastHTTPBenchmark(b, handler, "GET", "/v1/middleware")
}

// --- Raw fasthttp baseline ---
// Theoretical maximum: pure fasthttp handler with zero framework overhead.

func BenchmarkRawFastHttp_JSON(b *testing.B) {
	handler := func(ctx *fasthttp.RequestCtx) {
		ctx.SetContentType("application/json")
		ctx.SetBodyString(`{"message":"hello"}`)
	}
	runFastHTTPBenchmark(b, handler, "GET", "/json")
}

func BenchmarkRawFastHttp_PathParam(b *testing.B) {
	handler := func(ctx *fasthttp.RequestCtx) {
		path := ctx.URI().PathOriginal()
		var id []byte
		if i := len(path) - 1; i > 0 {
			for j := i; j >= 0; j-- {
				if path[j] == '/' {
					id = path[j+1:]
					break
				}
			}
		}
		ctx.SetContentType("application/json")
		ctx.SetBodyString(`{"id":"` + string(id) + `"}`)
	}
	runFastHTTPBenchmark(b, handler, "GET", "/path/42")
}

func BenchmarkRawFastHttp_QueryParam(b *testing.B) {
	handler := func(ctx *fasthttp.RequestCtx) {
		name := ctx.QueryArgs().Peek("name")
		ctx.SetContentType("application/json")
		ctx.SetBodyString(`{"name":"` + string(name) + `"}`)
	}
	runFastHTTPBenchmark(b, handler, "GET", "/query?name=hello")
}
