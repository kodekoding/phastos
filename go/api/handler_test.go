package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kodekoding/phastos/v2/go/common"
)

func TestWrittenResponseWriter(t *testing.T) {
	t.Run("WriteHeader sets written flag", func(t *testing.T) {
		inner := httptest.NewRecorder()
		w := writtenWriterPool.Get().(*WrittenResponseWriter) //nolint:errcheck
		w.ResponseWriter = inner
		w.written = false

		w.WriteHeader(http.StatusCreated)
		assert.True(t, w.Written())
		assert.Equal(t, http.StatusCreated, inner.Code)

		ReleaseWrittenResponseWriter(w)
	})

	t.Run("Write sets written flag", func(t *testing.T) {
		inner := httptest.NewRecorder()
		w := writtenWriterPool.Get().(*WrittenResponseWriter) //nolint:errcheck
		w.ResponseWriter = inner
		w.written = false

		n, err := w.Write([]byte("hello"))
		assert.NoError(t, err)
		assert.Equal(t, 5, n)
		assert.True(t, w.Written())

		ReleaseWrittenResponseWriter(w)
	})

	t.Run("Flush delegates to underlying flusher", func(t *testing.T) {
		inner := httptest.NewRecorder()
		w := writtenWriterPool.Get().(*WrittenResponseWriter) //nolint:errcheck
		w.ResponseWriter = inner
		w.written = false

		// Flush should not panic even if underlying doesn't support it
		w.Flush()

		ReleaseWrittenResponseWriter(w)
	})

	t.Run("Written returns false initially", func(t *testing.T) {
		inner := httptest.NewRecorder()
		w := writtenWriterPool.Get().(*WrittenResponseWriter) //nolint:errcheck
		w.ResponseWriter = inner
		w.written = false

		assert.False(t, w.Written())

		ReleaseWrittenResponseWriter(w)
	})
}

func TestInitHandler_RequestIdFromHeader(t *testing.T) {
	inner := http.NewServeMux()
	inner.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := InitHandler(inner)

	t.Run("uses X-Request-Id from header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-Id", "test-request-id")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("uses X-Request-ID as fallback", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", "test-request-id-2")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("generates request ID when none provided", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should have set a request ID in the request header
		reqId := req.Header.Get(common.RequestIDHeader)
		assert.NotEmpty(t, reqId)
	})
}
