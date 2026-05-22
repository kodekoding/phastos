package benchmark

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/valyala/fasthttp"
)

// JSONResponse is the common response struct for all frameworks
type JSONResponse struct {
	Message string `json:"message"`
}

// runNetHTTPBenchmark runs a benchmark against an http.Handler
func runNetHTTPBenchmark(b *testing.B, handler http.Handler, method, path string) {
	b.Helper()
	req := httptest.NewRequest(method, path, nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

// runNetHTTPBenchmarkWithBody runs a benchmark with a JSON body
func runNetHTTPBenchmarkWithBody(b *testing.B, handler http.Handler, method, path string, body interface{}) {
	b.Helper()
	bodyBytes, _ := json.Marshal(body)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(method, path, io.NopCloser(bytes.NewReader(bodyBytes)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

// runFastHTTPBenchmark runs a benchmark against a fasthttp.RequestHandler
func runFastHTTPBenchmark(b *testing.B, handler fasthttp.RequestHandler, method, path string) {
	b.Helper()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var ctx fasthttp.RequestCtx
		var req fasthttp.Request
		req.Header.SetMethod(method)
		req.SetRequestURI(path)
		ctx.Init(&req, nil, nil)
		handler(&ctx)
	}
}
