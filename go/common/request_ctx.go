package common

import "context"

// requestIDKeyType is an unexported type used as context key.
// Kept for backward compatibility with existing code that
// stores request_id via context.WithValue.
type requestIDKeyType struct{}

var requestIDKey requestIDKeyType

// WithRequestID stores the request ID in the context using a typed key.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		return v.(string) //nolint:errcheck
	}
	// Legacy string-key compatibility
	if v := ctx.Value(RequestIdContextKey); v != nil {
		return v.(string) //nolint:errcheck
	}
	return ""
}

// RequestIDHeader is the HTTP header used to pass the request ID.
const RequestIDHeader = "X-Request-Id"

// GetRequestIDFromHeader retrieves the request ID from the HTTP header.
// This avoids context.Value chain traversal entirely.
func GetRequestIDFromHeader(r interface{ Header() map[string][]string }) string {
	if h := r.Header(); h != nil {
		if v := h[RequestIDHeader]; len(v) > 0 {
			return v[0]
		}
		if v := h["X-Request-ID"]; len(v) > 0 {
			return v[0]
		}
	}
	return ""
}
