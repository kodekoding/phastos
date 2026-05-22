package benchmark

import (
	"encoding/json"
	"net/http"
	"testing"
)

// --- Standard Library net/http Benchmarks ---

func stdlibJSONHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "hello"})
	}
}

func stdlibPathParamHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	}
}

func stdlibQueryParamHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"name": name})
	}
}

func stdlibMiddlewareHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "hello"})
	}
}

func setupStdlibMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /json", stdlibJSONHandler())
	mux.HandleFunc("GET /path/{id}", stdlibPathParamHandler())
	mux.HandleFunc("GET /query", stdlibQueryParamHandler())
	mux.HandleFunc("GET /middleware", stdlibMiddlewareHandler())
	return mux
}

func BenchmarkStdlib_JSON(b *testing.B) {
	mux := setupStdlibMux()
	runNetHTTPBenchmark(b, mux, "GET", "/json")
}

func BenchmarkStdlib_PathParam(b *testing.B) {
	mux := setupStdlibMux()
	runNetHTTPBenchmark(b, mux, "GET", "/path/42")
}

func BenchmarkStdlib_QueryParam(b *testing.B) {
	mux := setupStdlibMux()
	runNetHTTPBenchmark(b, mux, "GET", "/query?name=hello")
}

func BenchmarkStdlib_Middleware(b *testing.B) {
	handler := dummyMiddleware1(dummyMiddleware2(dummyMiddleware3(stdlibMiddlewareHandler())))
	runNetHTTPBenchmark(b, handler, "GET", "/middleware")
}
