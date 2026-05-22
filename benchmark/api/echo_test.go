package benchmark

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func setupEchoRouter() *echo.Echo {
	e := echo.New()

	e.GET("/json", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "hello"})
	})

	e.GET("/path/:id", func(c echo.Context) error {
		id := c.Param("id")
		return c.JSON(http.StatusOK, map[string]string{"id": id})
	})

	e.GET("/query", func(c echo.Context) error {
		name := c.QueryParam("name")
		return c.JSON(http.StatusOK, map[string]string{"name": name})
	})

	return e
}

func BenchmarkEcho_JSON(b *testing.B) {
	e := setupEchoRouter()
	req := httptest.NewRequest("GET", "/json", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_PathParam(b *testing.B) {
	e := setupEchoRouter()
	req := httptest.NewRequest("GET", "/path/42", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_QueryParam(b *testing.B) {
	e := setupEchoRouter()
	req := httptest.NewRequest("GET", "/query?name=hello", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_Middleware(b *testing.B) {
	e := echo.New()
	e.Use(echoMiddleware1, echoMiddleware2, echoMiddleware3)
	e.GET("/middleware", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "hello"})
	})
	req := httptest.NewRequest("GET", "/middleware", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

func echoMiddleware1(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("X-Mw1", "true")
		return next(c)
	}
}

func echoMiddleware2(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("X-Mw2", "true")
		return next(c)
	}
}

func echoMiddleware3(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("X-Mw3", "true")
		return next(c)
	}
}
