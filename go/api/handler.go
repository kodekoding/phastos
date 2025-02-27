package api

import (
	"context"
	"net/http"

	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/helper"
)

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

func (w *WrittenResponseWriter) Written() bool {
	return w.written
}

func InitHandler(router http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writtenResponseWriter := &WrittenResponseWriter{
			ResponseWriter: w,
			written:        false,
		}
		w = writtenResponseWriter

		// set request id and store to context
		requestId := r.Header.Get("X-Request-ID")
		uniqueRequestId := helper.GenerateRandomString(15)
		if requestId == "" {
			requestId = uniqueRequestId
		}
		ctx := context.WithValue(r.Context(), common.RequestIdContextKey, requestId)
		*r = *r.WithContext(ctx)

		router.ServeHTTP(w, r)
	})
}
