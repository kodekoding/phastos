package benchmark

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
)

func setupChiMux() *chi.Mux {
	r := chi.NewRouter()

	r.Get("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "hello"})
	})

	r.Get("/path/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	})

	r.Get("/query", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"name": name})
	})

	r.Get("/middleware", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "hello"})
	})

	return r
}

func BenchmarkChi_JSON(b *testing.B) {
	r := setupChiMux()
	runNetHTTPBenchmark(b, r, "GET", "/json")
}

func BenchmarkChi_PathParam(b *testing.B) {
	r := setupChiMux()
	runNetHTTPBenchmark(b, r, "GET", "/path/42")
}

func BenchmarkChi_QueryParam(b *testing.B) {
	r := setupChiMux()
	runNetHTTPBenchmark(b, r, "GET", "/query?name=hello")
}

func BenchmarkChi_Middleware(b *testing.B) {
	r := chi.NewRouter()
	r.Use(dummyMiddleware1, dummyMiddleware2, dummyMiddleware3)
	r.Get("/middleware", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "hello"})
	})
	runNetHTTPBenchmark(b, r, "GET", "/middleware")
}
