package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/helper"
	plog "github.com/kodekoding/phastos/v2/go/log"
)

// writtenWriterPool reduces allocations for WrittenResponseWriter.
var writtenWriterPool = sync.Pool{
	New: func() interface{} {
		return &WrittenResponseWriter{}
	},
}

type WrittenResponseWriter struct {
	http.ResponseWriter
	written bool
}

func (w *WrittenResponseWriter) WriteHeader(status int) {
	w.written = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *WrittenResponseWriter) Write(b []byte) (int, error) {
	w.written = true
	return w.ResponseWriter.Write(b)
}

// Flush implements http.Flusher interface for streaming responses (SSE, WebSocket, etc.)
func (w *WrittenResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *WrittenResponseWriter) Written() bool {
	return w.written
}

// ReleaseWrittenResponseWriter resets and returns a WrittenResponseWriter to the pool.
func ReleaseWrittenResponseWriter(w *WrittenResponseWriter) {
	w.ResponseWriter = nil
	w.written = false
	writtenWriterPool.Put(w)
}

func InitHandler(router http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writtenResponseWriter := writtenWriterPool.Get().(*WrittenResponseWriter) //nolint:errcheck
		writtenResponseWriter.ResponseWriter = w
		writtenResponseWriter.written = false
		w = writtenResponseWriter
		defer ReleaseWrittenResponseWriter(writtenResponseWriter)

		// Generate or use existing request ID.
		requestId := r.Header.Get("X-Request-Id")
		if requestId == "" {
			requestId = r.Header.Get("X-Request-ID")
		}
		if requestId == "" {
			requestId = helper.GenerateFastID()
		}
		// Store request ID in the header directly instead of context.
		// This avoids context.WithValue chain traversal that caused
		// 86% CPU overhead in profiling.
		r.Header.Set(common.RequestIDHeader, requestId)

		router.ServeHTTP(w, r)
	})
}

// skipLogPaths is set by App.Init() via setSkipLogPaths().
// It contains paths that should skip the requestLogger middleware entirely.
var skipLogPaths map[string]struct{}

// setSkipLogPaths is called by App.Init() to pass the configured skip paths
// into the requestLogger middleware closure.
func setSkipLogPaths(paths map[string]struct{}) {
	skipLogPaths = paths
}

func requestLogger(next http.Handler) http.Handler {
	// When zerolog is globally disabled (e.g. benchmarks), skip the
	// entire middleware — no logging, no ResponseRecorder, no overhead.
	if zerolog.GlobalLevel() == zerolog.Disabled {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Still set the X-Request-Id response header for downstream consumers.
			requestId := r.Header.Get(common.RequestIDHeader)
			if requestId == "" {
				requestId = r.Header.Get("X-Request-ID")
			}
			if requestId != "" {
				w.Header().Set("X-Request-Id", requestId)
			}
			next.ServeHTTP(w, r)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check configurable skip paths (/ping is always skipped)
		if r.URL.Path == "/ping" {
			next.ServeHTTP(w, r)
			return
		}
		if _, skip := skipLogPaths[r.URL.Path]; skip {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		log := plog.Get()

		// Read request ID from header (set by InitHandler).
		requestId := r.Header.Get(common.RequestIDHeader)
		if requestId == "" {
			requestId = r.Header.Get("X-Request-ID")
		}
		// register `X-Request-Id` to header response
		w.Header().Add("X-Request-Id", requestId)

		// update log context with embed request_id
		log.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("request_id", requestId)
		})

		respRecorder := NewResponseRecorder(w)

		// incoming request
		log.
			Info().
			Str("method", r.Method).
			Str("url", r.URL.RequestURI()).
			Str("user_agent", r.UserAgent()).
			Msg("Incoming Request")

		defer func() {
			// final log — use the same logger instance (already has request_id)
			log.
				Info().
				Str("method", r.Method).
				Str("url", r.URL.RequestURI()).
				Str("user_agent", r.UserAgent()).
				Int("status_code", respRecorder.StatusCode).
				Dur("elapsed_ms", time.Since(start)).
				Msg("Request Finished")
		}()
		next.ServeHTTP(respRecorder, r)
	})
}
