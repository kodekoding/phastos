package middlewares

import (
	"net/http"
	"os"

	"github.com/kodekoding/phastos/v2/go/api"
	"github.com/kodekoding/phastos/v2/go/common"
)

func StaticAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceId := r.Header.Get(common.RequestIDHeader)
		if traceId == "" {
			traceId = r.Header.Get("X-Request-ID")
		}

		expectedToken := os.Getenv(common.EnvServiceSecret)
		var token string
		if authHeader := r.Header.Get(common.HeaderSecret); authHeader != "" {
			token = authHeader
		} else {
			unauthorizedInvalidToken(w, traceId)
			return
		}

		if expectedToken != token {
			unauthorizedInvalidToken(w, traceId)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func unauthorizedInvalidToken(w http.ResponseWriter, traceId string) {
	errUnauthorized := api.Unauthorized(common.ErrInvalidTokenMessage, common.ErrInvalidTokenCode)
	errUnauthorized.TraceId = traceId
	api.NewResponse().SetError(errUnauthorized).Send(w)
}
