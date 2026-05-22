package middlewares

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/kodekoding/phastos/v2/go/api"
	"golang.org/x/time/rate"
)

// KeyExtractor returns the rate-limit bucket key for a request.
type KeyExtractor func(r *http.Request) string

// RateLimiterOption configures the rate limiter.
type RateLimiterOption func(*rateLimiter)

type rateLimiter struct {
	limiter      *rate.Limiter
	limiters     *sync.Map
	keyExtractor KeyExtractor
	skipPaths    map[string]struct{}
	msg          string
	code         string
}

// NewRateLimiter returns an http.Handler middleware that rate-limits requests
// using a token-bucket algorithm per bucket key (default = IP address).
func NewRateLimiter(opts ...RateLimiterOption) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		limiter:      rate.NewLimiter(rate.Limit(10), 20),
		limiters:     &sync.Map{},
		keyExtractor: defaultKeyExtractor(),
		skipPaths:    map[string]struct{}{},
		msg:          "rate limit exceeded",
		code:         "RATE_LIMITED",
	}

	for _, opt := range opts {
		opt(rl)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := rl.skipPaths[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}

			key := rl.keyExtractor(r)
			var lim *rate.Limiter
			v, loaded := rl.limiters.LoadOrStore(key, rate.NewLimiter(rl.limiter.Limit(), rl.limiter.Burst()))
			lim = v.(*rate.Limiter) //nolint:errcheck
			_ = loaded

			if !lim.Allow() {
				err := api.TooManyRequest(rl.msg, rl.code)
				api.NewResponse().SetHTTPError(err).Send(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// WithRate sets the request-per-second limit and burst size.
func WithRate(rps float64, burst int) RateLimiterOption {
	return func(rl *rateLimiter) {
		rl.limiter = rate.NewLimiter(rate.Limit(rps), burst)
	}
}

// WithKeyExtractor replaces the default IP-based key extractor.
func WithKeyExtractor(fn KeyExtractor) RateLimiterOption {
	return func(rl *rateLimiter) {
		rl.keyExtractor = fn
	}
}

// WithSkipPaths disables rate limiting for the given exact paths.
func WithSkipPaths(paths ...string) RateLimiterOption {
	return func(rl *rateLimiter) {
		for _, p := range paths {
			rl.skipPaths[p] = struct{}{}
		}
	}
}

// WithMessage sets the error message and error code returned on limit.
func WithMessage(msg, code string) RateLimiterOption {
	return func(rl *rateLimiter) {
		rl.msg = msg
		rl.code = code
	}
}

// defaultKeyExtractor returns the client IP as the limit key.
func defaultKeyExtractor() KeyExtractor {
	return func(r *http.Request) string {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.Header.Get("X-Real-Ip")
		}
		if ip == "" {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				return r.RemoteAddr
			}
			ip = host
		}
		// If X-Forwarded-For contains multiple IPs, use the first one.
		if idx := strings.Index(ip, ","); idx != -1 {
			ip = strings.TrimSpace(ip[:idx])
		}
		return ip
	}
}
