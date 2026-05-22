package benchmark

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

func setupFiberApp() *fiber.App {
	app := fiber.New(fiber.Config{
		Prefork:               false,
		DisableDefaultDate:    true,
		DisableDefaultContentType: true,
	})

	app.Get("/json", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "hello"})
	})

	app.Get("/path/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		return c.JSON(fiber.Map{"id": id})
	})

	app.Get("/query", func(c *fiber.Ctx) error {
		name := c.Query("name")
		return c.JSON(fiber.Map{"name": name})
	})

	return app
}

func BenchmarkFiber_JSON(b *testing.B) {
	app := setupFiberApp()
	handler := app.Handler()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var ctx fasthttp.RequestCtx
		var req fasthttp.Request
		req.Header.SetMethod("GET")
		req.SetRequestURI("/json")
		ctx.Init(&req, nil, nil)
		handler(&ctx)
	}
}

func BenchmarkFiber_PathParam(b *testing.B) {
	app := setupFiberApp()
	handler := app.Handler()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var ctx fasthttp.RequestCtx
		var req fasthttp.Request
		req.Header.SetMethod("GET")
		req.SetRequestURI("/path/42")
		ctx.Init(&req, nil, nil)
		handler(&ctx)
	}
}

func BenchmarkFiber_QueryParam(b *testing.B) {
	app := setupFiberApp()
	handler := app.Handler()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var ctx fasthttp.RequestCtx
		var req fasthttp.Request
		req.Header.SetMethod("GET")
		req.SetRequestURI("/query?name=hello")
		ctx.Init(&req, nil, nil)
		handler(&ctx)
	}
}

func BenchmarkFiber_Middleware(b *testing.B) {
	app := fiber.New(fiber.Config{
		Prefork:               false,
		DisableDefaultDate:    true,
		DisableDefaultContentType: true,
	})
	app.Use(fiberMiddleware1, fiberMiddleware2, fiberMiddleware3)
	app.Get("/middleware", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "hello"})
	})
	handler := app.Handler()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var ctx fasthttp.RequestCtx
		var req fasthttp.Request
		req.Header.SetMethod("GET")
		req.SetRequestURI("/middleware")
		ctx.Init(&req, nil, nil)
		handler(&ctx)
	}
}

func fiberMiddleware1(c *fiber.Ctx) error {
	c.Set("X-Mw1", "true")
	return c.Next()
}

func fiberMiddleware2(c *fiber.Ctx) error {
	c.Set("X-Mw2", "true")
	return c.Next()
}

func fiberMiddleware3(c *fiber.Ctx) error {
	c.Set("X-Mw3", "true")
	return c.Next()
}
