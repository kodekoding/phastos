package context

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/stretchr/testify/assert"
)

func TestSetAndGetJWT(t *testing.T) {
	t.Run("should set and get JWT data from request context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		jwtData := &entity.JWTClaimData{
			Data:  map[string]interface{}{"user_id": 1},
			Token: "test-token",
		}

		SetJWT(req, jwtData)

		result := GetJWT(req.Context())
		assert.NotNil(t, result)
		assert.Equal(t, "test-token", result.Token)
		data := result.Data.(map[string]interface{})
		assert.Equal(t, 1, data["user_id"])
	})

	t.Run("should return nil when JWT not set", func(t *testing.T) {
		ctx := context.Background()
		result := GetJWT(ctx)
		assert.Nil(t, result)
	})

	t.Run("should return nil when context value is wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), JwtContext{}, "not-jwt-data")
		result := GetJWT(ctx)
		assert.Nil(t, result)
	})
}
