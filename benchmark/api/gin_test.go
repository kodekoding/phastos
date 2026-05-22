package benchmark

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupGinRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/json", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "hello"})
	})

	r.GET("/path/:id", func(c *gin.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, gin.H{"id": id})
	})

	r.GET("/query", func(c *gin.Context) {
		name := c.Query("name")
		c.JSON(http.StatusOK, gin.H{"name": name})
	})

	return r
}

func BenchmarkGin_JSON(b *testing.B) {
	r := setupGinRouter()
	runNetHTTPBenchmark(b, r, "GET", "/json")
}

func BenchmarkGin_PathParam(b *testing.B) {
	r := setupGinRouter()
	runNetHTTPBenchmark(b, r, "GET", "/path/42")
}

func BenchmarkGin_QueryParam(b *testing.B) {
	r := setupGinRouter()
	runNetHTTPBenchmark(b, r, "GET", "/query?name=hello")
}

func BenchmarkGin_Middleware(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginMiddleware1, ginMiddleware2, ginMiddleware3)
	r.GET("/middleware", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "hello"})
	})
	runNetHTTPBenchmark(b, r, "GET", "/middleware")
}

func ginMiddleware1(c *gin.Context) {
	c.Header("X-Mw1", "true")
	c.Next()
}

func ginMiddleware2(c *gin.Context) {
	c.Header("X-Mw2", "true")
	c.Next()
}

func ginMiddleware3(c *gin.Context) {
	c.Header("X-Mw3", "true")
	c.Next()
}
