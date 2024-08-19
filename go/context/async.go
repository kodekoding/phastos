package context

import (
	"context"
	"github.com/kodekoding/phastos/v2/go/cache"
	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/kodekoding/phastos/v2/go/notifications"
)

func CreateAsyncContext(ctx context.Context) context.Context {
	asyncContext := context.Background()
	notifCtx := ctx.Value(entity.NotifPlatformContext{}).(notifications.Platforms)
	cacheCtx := ctx.Value(entity.CacheContext{}).(cache.Caches)
	jwtCtx := ctx.Value(JwtContext{}).(*entity.JWTClaimData)

	asyncContext = context.WithValue(asyncContext, entity.NotifPlatformContext{}, notifCtx)
	asyncContext = context.WithValue(asyncContext, entity.CacheContext{}, cacheCtx)
	asyncContext = context.WithValue(asyncContext, JwtContext{}, jwtCtx)
	return asyncContext
}
