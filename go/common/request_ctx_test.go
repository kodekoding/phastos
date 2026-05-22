package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "test-req-123")
	val := GetRequestID(ctx)
	assert.Equal(t, "test-req-123", val)
}

func TestGetRequestIDEmpty(t *testing.T) {
	ctx := context.Background()
	val := GetRequestID(ctx)
	assert.Equal(t, "", val)
}

func TestGetRequestIDFromLegacyStringKey(t *testing.T) {
	ctx := context.Background()
	// Set using the old string key for backward compatibility
	ctx = context.WithValue(ctx, RequestIdContextKey, "legacy-id")
	val := GetRequestID(ctx)
	assert.Equal(t, "legacy-id", val)
}

func TestGetRequestIDTypedKeyTakesPrecedence(t *testing.T) {
	ctx := context.Background()
	// Set both typed key and legacy string key
	ctx = WithRequestID(ctx, "typed-id")
	ctx = context.WithValue(ctx, RequestIdContextKey, "legacy-id")
	val := GetRequestID(ctx)
	// Typed key should be found first
	assert.Equal(t, "typed-id", val)
}

func TestGetRequestIDFromHeader(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string][]string
		expected string
	}{
		{
			name:     "X-Request-Id header",
			headers:  map[string][]string{"X-Request-Id": {"req-123"}},
			expected: "req-123",
		},
		{
			name:     "X-Request-ID header (alternate casing)",
			headers:  map[string][]string{"X-Request-ID": {"req-456"}},
			expected: "req-456",
		},
		{
			name:     "both headers present, X-Request-Id takes priority",
			headers:  map[string][]string{"X-Request-Id": {"first"}, "X-Request-ID": {"second"}},
			expected: "first",
		},
		{
			name:     "no headers",
			headers:  map[string][]string{},
			expected: "",
		},
		{
			name:     "nil headers",
			headers:  nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock request-like object
			mockReq := &mockHeaderer{headers: tt.headers}
			val := GetRequestIDFromHeader(mockReq)
			assert.Equal(t, tt.expected, val)
		})
	}
}

func TestRequestIDHeader(t *testing.T) {
	assert.Equal(t, "X-Request-Id", RequestIDHeader)
}

// mockHeaderer implements the interface{ Header() map[string][]string }
type mockHeaderer struct {
	headers map[string][]string
}

func (m *mockHeaderer) Header() map[string][]string {
	return m.headers
}
