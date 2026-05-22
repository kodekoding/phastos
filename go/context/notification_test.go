package context

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/kodekoding/phastos/v2/go/notifications"
	"github.com/stretchr/testify/assert"
)

// stubPlatforms implements notifications.Platforms for testing.
type stubPlatforms struct {
	slack    notifications.Action
	telegram notifications.Action
}

func (s *stubPlatforms) Telegram() notifications.Action        { return s.telegram }
func (s *stubPlatforms) Slack() notifications.Action           { return s.slack }
func (s *stubPlatforms) GetAllPlatform() []notifications.Action { return nil }
func (s *stubPlatforms) WrapToHandler(next http.Handler) http.Handler {
	return next
}
func (s *stubPlatforms) WrapToContext(ctx context.Context) context.Context {
	return ctx
}

func TestSetNotifAndGetNotif(t *testing.T) {
	t.Run("should set and get notif platform from request context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		platform := &stubPlatforms{}

		SetNotif(req, platform)

		result := GetNotif(req.Context())
		assert.NotNil(t, result)
		assert.Equal(t, platform, result)
	})
}

func TestGetNotif_NoNotifInContext(t *testing.T) {
	t.Run("should return nil when no notif in context", func(t *testing.T) {
		ctx := context.Background()
		result := GetNotif(ctx)
		assert.Nil(t, result)
	})
}

func TestGetNotif_WrongType(t *testing.T) {
	t.Run("should return nil when context value is wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, "not-platforms")
		result := GetNotif(ctx)
		assert.Nil(t, result)
	})
}

func TestSetNotifDestinationAndGetNotifDestination(t *testing.T) {
	t.Run("should set and get notif destination from request context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		SetNotifDestination(req, "#general")

		result := GetNotifDestination(req.Context())
		assert.Equal(t, "#general", result)
	})

	t.Run("should overwrite destination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		SetNotifDestination(req, "#first")
		SetNotifDestination(req, "#second")

		result := GetNotifDestination(req.Context())
		assert.Equal(t, "#second", result)
	})
}

func TestGetNotifDestination_NoDestinationInContext(t *testing.T) {
	t.Run("should return empty string when no destination in context", func(t *testing.T) {
		ctx := context.Background()
		result := GetNotifDestination(ctx)
		assert.Equal(t, "", result)
	})
}

func TestGetNotifDestination_WrongType(t *testing.T) {
	t.Run("should return empty string when context value is wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), entity.NotifDestinationContext{}, 12345)
		result := GetNotifDestination(ctx)
		assert.Equal(t, "", result)
	})
}

func TestSetNotif_PreservesExistingContext(t *testing.T) {
	t.Run("should preserve existing context values when setting notif", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		// Set JWT first
		jwtData := &entity.JWTClaimData{Data: "test", Token: "token"}
		SetJWT(req, jwtData)

		// Now set notif
		platform := &stubPlatforms{}
		SetNotif(req, platform)

		// Both should be retrievable
		jwtResult := GetJWT(req.Context())
		assert.NotNil(t, jwtResult)
		assert.Equal(t, "token", jwtResult.Token)

		notifResult := GetNotif(req.Context())
		assert.NotNil(t, notifResult)
	})
}

func TestSetNotifDestination_PreservesExistingContext(t *testing.T) {
	t.Run("should preserve existing context values when setting destination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		jwtData := &entity.JWTClaimData{Data: "test", Token: "token"}
		SetJWT(req, jwtData)

		SetNotifDestination(req, "#alerts")

		jwtResult := GetJWT(req.Context())
		assert.NotNil(t, jwtResult)

		destResult := GetNotifDestination(req.Context())
		assert.Equal(t, "#alerts", destResult)
	})
}
