package api

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/helper"
	plog "github.com/kodekoding/phastos/v2/go/log"
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

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log := plog.Get()
		ctx := r.Context()

		requestId := ctx.Value(common.RequestIdContextKey).(string)
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

		r = r.WithContext(log.WithContext(r.Context()))
		defer func() {
			// final log
			finalLog := plog.Ctx(r.Context())
			finalLog.
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
