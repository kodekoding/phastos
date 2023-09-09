package middlewares

import (
	"net/http"
	"os"

	"github.com/kodekoding/phastos/v2/go/api"
	"github.com/kodekoding/phastos/v2/go/common"
)

func StaticAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceIdCtx, _ := r.Context().Value(common.TraceIdKeyContextStr).(string)

		expectedToken := os.Getenv(common.EnvServiceSecret)
		var token = ""
		if authHeader := r.Header.Get(common.HeaderSecret); authHeader != "" {
			token = authHeader
		} else {
			unauthorizedInvalidToken(w, traceIdCtx)
			return
		}

		if expectedToken != token {
			unauthorizedInvalidToken(w, traceIdCtx)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func unauthorizedInvalidToken(w http.ResponseWriter, traceId string) {
	errUnauthorized := api.Unauthorized(common.ErrInvalidTokenMessage, common.ErrInvalidTokenCode)
	errUnauthorized.SetTraceId(traceId)
	api.NewResponse().SetError(errUnauthorized).Send(w)
}
