package entity

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

func TestContextTypes(t *testing.T) {
	// Test that the context types can be used as context keys
	_ = NotifPlatformContext{}
	_ = CacheContext{}
	_ = NotifDestinationContext{}
}

func TestCacheContextKey(t *testing.T) {
	key := CacheContext{}
	assert.NotNil(t, key)
	// Verify it's a struct (used as context key)
	assert.Equal(t, CacheContext{}, CacheContext{})
}

func TestNotifPlatformContextKey(t *testing.T) {
	key := NotifPlatformContext{}
	assert.NotNil(t, key)
}

func TestNotifDestinationContextKey(t *testing.T) {
	key := NotifDestinationContext{}
	assert.NotNil(t, key)
}

func TestJWTClaimData(t *testing.T) {
	data := JWTClaimData{
		Data:  map[string]string{"user": "test"},
		Token: "test-token",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	assert.Equal(t, "test-token", data.Token)
	assert.NotNil(t, data.Data)
	assert.NotNil(t, data.ExpiresAt)
}

func TestJWTClaimDataJSON(t *testing.T) {
	data := JWTClaimData{
		Data:  "user-data",
		Token: "jwt-token",
	}
	b, err := json.Marshal(data)
	assert.NoError(t, err)
	assert.Contains(t, string(b), "jwt-token")

	var result JWTClaimData
	err = json.Unmarshal(b, &result)
	assert.NoError(t, err)
	assert.Equal(t, "jwt-token", result.Token)
}

func TestJWTClaimDataWithNilData(t *testing.T) {
	data := JWTClaimData{
		Data:  nil,
		Token: "token",
	}
	assert.Nil(t, data.Data)
	assert.Equal(t, "token", data.Token)
}
