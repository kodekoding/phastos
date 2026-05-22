package api

import (
	"encoding/json"
	"os"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"golang.org/x/time/rate"

	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/entity"
)

// --- FastStaticAuth is the fasthttp-native version of StaticAuth middleware ---

func FastStaticAuth(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		traceId := string(ctx.Request.Header.Peek(common.RequestIDHeader))
		if traceId == "" {
			traceId = string(ctx.Response.Header.Peek(common.RequestIDHeader))
		}

		expectedToken := os.Getenv(common.EnvServiceSecret)
		var token string
		if authHeader := string(ctx.Request.Header.Peek(common.HeaderSecret)); authHeader != "" {
			token = authHeader
		} else {
			fastUnauthorizedInvalidToken(ctx, traceId)
			return
		}

		if expectedToken != token {
			fastUnauthorizedInvalidToken(ctx, traceId)
			return
		}

		next(ctx)
	}
}

func fastUnauthorizedInvalidToken(ctx *fasthttp.RequestCtx, traceId string) {
	errUnauthorized := Unauthorized(common.ErrInvalidTokenMessage, common.ErrInvalidTokenCode)
	errUnauthorized.TraceId = traceId
	response := NewResponse().SetError(errUnauthorized)
	fastSendResponse(ctx, response)
	ReleaseResponse(response)
}

// --- FastJWTAuth is the fasthttp-native version of JWTAuth middleware ---

func FastJWTAuth(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		traceId := string(ctx.Request.Header.Peek(common.RequestIDHeader))
		if traceId == "" {
			traceId = string(ctx.Response.Header.Peek(common.RequestIDHeader))
		}

		var token string
		if authHeader := string(ctx.Request.Header.Peek("Authorization")); authHeader != "" {
			token = strings.Replace(authHeader, "Bearer ", "", 1)
		} else {
			fastJWTUnauthorized(ctx, traceId, "missing authorization header", "INVALID_TOKEN")
			return
		}

		if os.Getenv(common.EnvJWTSigningKey) == "" {
			err := errors.New("JWT Signing Key is nil")
			newError := Unauthorized(err.Error(), "INVALID_KEY")
			newError.TraceId = traceId
			response := NewResponse().SetError(newError)
			fastSendResponse(ctx, response)
			ReleaseResponse(response)
			return
		}

		var keyFunc jwt.Keyfunc = func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != "HS256" {
				return nil, errors.Errorf("unexpected jwt signing method=%v", token.Header["alg"])
			}
			return []byte(os.Getenv(common.EnvJWTSigningKey)), nil
		}

		tokenClient := strings.TrimSpace(token)
		data := jwt.MapClaims{}

		tokenData, errToken := jwt.ParseWithClaims(tokenClient, data, keyFunc)
		if errToken != nil {
			fastJWTUnauthorized(ctx, traceId, errToken.Error(), "INVALID_CLAIMS")
			return
		}
		if !tokenData.Valid {
			fastJWTUnauthorized(ctx, traceId, "Token is not valid", "TOKEN_NOT_VALID")
			return
		}

		claimByte, _ := json.Marshal(tokenData.Claims)
		var result entity.JWTClaimData
		if err := json.Unmarshal(claimByte, &result); err != nil {
			fastJWTUnauthorized(ctx, traceId, "invalid struct claim", "INVALID_STRUCT_CLAIM")
			return
		}

		result.Token = token
		ctx.SetUserValue("jwt_claim", &result)

		next(ctx)
	}
}

func fastJWTUnauthorized(ctx *fasthttp.RequestCtx, traceId, message, code string) {
	errResp := Unauthorized(message, code)
	errResp.TraceId = traceId
	response := NewResponse().SetError(errResp)
	fastSendResponse(ctx, response)
	ReleaseResponse(response)
}

// --- FastRateLimiter is the fasthttp-native version of RateLimiter ---

type FastKeyExtractor func(ctx *fasthttp.RequestCtx) string

type FastRateLimiterOption func(*fastRateLimiter)

type fastRateLimiter struct {
	limiter      *rate.Limiter
	limiters     *sync.Map
	keyExtractor FastKeyExtractor
	skipPaths    map[string]struct{}
	msg          string
	code         string
}

// FastNewRateLimiter returns a fasthttp middleware that rate-limits requests
// using a token-bucket algorithm per bucket key (default = IP address).
func FastNewRateLimiter(opts ...FastRateLimiterOption) FastMiddleware {
	rl := &fastRateLimiter{
		limiter:      rate.NewLimiter(rate.Limit(10), 20),
		limiters:     &sync.Map{},
		keyExtractor: fastDefaultKeyExtractor(),
		skipPaths:    map[string]struct{}{},
		msg:          "rate limit exceeded",
		code:         "RATE_LIMITED",
	}

	for _, opt := range opts {
		opt(rl)
	}

	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			if _, ok := rl.skipPaths[string(ctx.URI().Path())]; ok {
				next(ctx)
				return
			}

			key := rl.keyExtractor(ctx)
			v, _ := rl.limiters.LoadOrStore(key, rate.NewLimiter(rl.limiter.Limit(), rl.limiter.Burst()))
			lim := v.(*rate.Limiter) //nolint:errcheck

			if !lim.Allow() {
				err := TooManyRequest(rl.msg, rl.code)
				response := NewResponse().SetHTTPError(err)
				fastSendResponse(ctx, response)
				ReleaseResponse(response)
				return
			}

			next(ctx)
		}
	}
}

func fastDefaultKeyExtractor() FastKeyExtractor {
	return func(ctx *fasthttp.RequestCtx) string {
		ip := string(ctx.Request.Header.Peek("X-Forwarded-For"))
		if ip == "" {
			ip = string(ctx.Request.Header.Peek("X-Real-Ip"))
		}
		if ip == "" {
			ip = ctx.RemoteIP().String()
		}
		if idx := strings.Index(ip, ","); idx != -1 {
			ip = strings.TrimSpace(ip[:idx])
		}
		return ip
	}
}
