package middlewares

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/v2/go/api"
	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/entity"
)

type JWTConfig struct {
}

func JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		traceIdCtx, _ := r.Context().Value(common.TraceIdKeyContextStr).(string)

		var token = ""
		if authHeader := r.Header.Get("Authorization"); authHeader != "" {
			token = strings.Replace(authHeader, "Bearer ", "", 1)
		} else {
			unauthorizedInvalidToken(w, traceIdCtx)
			return
		}

		if os.Getenv(common.EnvJWTSigningKey) == "" {
			err := errors.New("JWT Signing Key is nil")
			newError := api.Unauthorized(err.Error(), "INVALID_KEY")
			newError.SetTraceId(traceIdCtx)
			api.NewResponse().SetError(newError).Send(w)

			return
		}

		var keyFunc jwt.Keyfunc
		keyFunc = func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != "HS256" {
				return nil, fmt.Errorf("unexpected jwt signing method=%v", token.Header["alg"])
			}
			return []byte(os.Getenv(common.EnvJWTSigningKey)), nil
		}

		tokenClient := strings.TrimSpace(token)
		data := jwt.MapClaims{}

		var errToken error
		tokenData, errToken := jwt.ParseWithClaims(tokenClient, data, keyFunc)
		if errToken != nil {
			invalidClaim := api.Unauthorized(errToken.Error(), "INVALID_CLAIMS")
			invalidClaim.SetTraceId(traceIdCtx)
			api.NewResponse().SetError(invalidClaim).Send(w)

			return
		}
		if !tokenData.Valid {
			invalidTokenError := api.Unauthorized("Token is not valid", "TOKEN_NOT_VALID")
			invalidTokenError.SetTraceId(traceIdCtx)
			api.NewResponse().SetError(invalidTokenError).Send(w)

			return
		}

		claimByte, _ := json.Marshal(tokenData.Claims)
		var result entity.JWTClaimData
		if err := json.Unmarshal(claimByte, &result); err != nil {
			invalidClaim := api.Unauthorized("invalid struct claim", "INVALID_STRUCT_CLAIM")
			invalidClaim.SetTraceId(traceIdCtx)
			api.NewResponse().SetError(invalidClaim).Send(w)

			return
		}

		result.Token = token

		context.SetJWT(r, &result)

		next.ServeHTTP(w, r)
	})
}
