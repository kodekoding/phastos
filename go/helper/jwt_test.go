package helper

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/stretchr/testify/assert"
)

func TestGenerateJWTToken(t *testing.T) {
	// Set up JWT signing key for all tests
	originalKey := os.Getenv("JWT_SIGNING_KEY")
	os.Setenv("JWT_SIGNING_KEY", "test-signing-key-for-unit-tests")
	defer func() {
		if originalKey == "" {
			os.Unsetenv("JWT_SIGNING_KEY")
		} else {
			os.Setenv("JWT_SIGNING_KEY", originalKey)
		}
	}()

	t.Run("should generate valid JWT token with default expiry", func(t *testing.T) {
		data := map[string]interface{}{"user_id": 123}
		token, err := GenerateJWTToken(data)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		// Verify the token can be parsed
		parsedToken, parseErr := jwt.ParseWithClaims(token, &entity.JWTClaimData{}, func(t *jwt.Token) (interface{}, error) {
			return []byte("test-signing-key-for-unit-tests"), nil
		})
		assert.NoError(t, parseErr)
		assert.True(t, parsedToken.Valid)

		claims, ok := parsedToken.Claims.(*entity.JWTClaimData)
		assert.True(t, ok)
		assert.NotNil(t, claims.Data)
	})

	t.Run("should generate valid JWT token with custom expiry", func(t *testing.T) {
		data := "test-user"
		token, err := GenerateJWTToken(data, 1*time.Hour)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		parsedToken, parseErr := jwt.ParseWithClaims(token, &entity.JWTClaimData{}, func(t *jwt.Token) (interface{}, error) {
			return []byte("test-signing-key-for-unit-tests"), nil
		})
		assert.NoError(t, parseErr)
		assert.True(t, parsedToken.Valid)

		claims, ok := parsedToken.Claims.(*entity.JWTClaimData)
		assert.True(t, ok)

		// Verify expiry is approximately 1 hour from now
		expectedExpiry := time.Now().Add(1 * time.Hour)
		assert.WithinDuration(t, expectedExpiry, claims.ExpiresAt.Time, 5*time.Second)
	})

	t.Run("should use default issuer when JWT_ISSUER is not set", func(t *testing.T) {
		os.Unsetenv("JWT_ISSUER")
		data := "test"
		token, err := GenerateJWTToken(data)
		assert.NoError(t, err)

		parsedToken, parseErr := jwt.ParseWithClaims(token, &entity.JWTClaimData{}, func(t *jwt.Token) (interface{}, error) {
			return []byte("test-signing-key-for-unit-tests"), nil
		})
		assert.NoError(t, parseErr)

		claims, ok := parsedToken.Claims.(*entity.JWTClaimData)
		assert.True(t, ok)
		assert.Equal(t, "phastos", claims.Issuer)
	})

	t.Run("should use custom issuer when JWT_ISSUER is set", func(t *testing.T) {
		os.Setenv("JWT_ISSUER", "custom-issuer")
		defer os.Unsetenv("JWT_ISSUER")

		data := "test"
		token, err := GenerateJWTToken(data)
		assert.NoError(t, err)

		parsedToken, parseErr := jwt.ParseWithClaims(token, &entity.JWTClaimData{}, func(t *jwt.Token) (interface{}, error) {
			return []byte("test-signing-key-for-unit-tests"), nil
		})
		assert.NoError(t, parseErr)

		claims, ok := parsedToken.Claims.(*entity.JWTClaimData)
		assert.True(t, ok)
		assert.Equal(t, "custom-issuer", claims.Issuer)
	})

	t.Run("should handle empty signing key gracefully", func(t *testing.T) {
		os.Unsetenv("JWT_SIGNING_KEY")
		data := "test"
		token, err := GenerateJWTToken(data)
		// HS256 with empty key still produces a token (not an error),
		// but the token is insecure
		assert.NotEmpty(t, token)
		assert.NoError(t, err)

		// Restore key for subsequent tests
		os.Setenv("JWT_SIGNING_KEY", "test-signing-key-for-unit-tests")
	})

	t.Run("should handle struct data in token", func(t *testing.T) {
		type userData struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		data := userData{ID: 42, Name: "John"}
		token, err := GenerateJWTToken(data)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		parsedToken, parseErr := jwt.ParseWithClaims(token, &entity.JWTClaimData{}, func(t *jwt.Token) (interface{}, error) {
			return []byte("test-signing-key-for-unit-tests"), nil
		})
		assert.NoError(t, parseErr)
		assert.True(t, parsedToken.Valid)
	})

	t.Run("should use HS256 signing method", func(t *testing.T) {
		data := "test"
		token, err := GenerateJWTToken(data)
		assert.NoError(t, err)

		parsedToken, parseErr := jwt.ParseWithClaims(token, &entity.JWTClaimData{}, func(t *jwt.Token) (interface{}, error) {
			return []byte("test-signing-key-for-unit-tests"), nil
		})
		assert.NoError(t, parseErr)
		assert.Equal(t, jwt.SigningMethodHS256, parsedToken.Method)
	})
}
