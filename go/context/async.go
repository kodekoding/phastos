package context

import (
	"context"
	"github.com/kodekoding/phastos/v2/go/cache"
	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/kodekoding/phastos/v2/go/notifications"
)

func CreateAsyncContext(ctx context.Context) context.Context {
	asyncContext := context.Background()
	if notifCtx, exists := ctx.Value(entity.NotifPlatformContext{}).(notifications.Platforms); exists {
		asyncContext = context.WithValue(asyncContext, entity.NotifPlatformContext{}, notifCtx)
	}
	if cacheCtx, exists := ctx.Value(entity.CacheContext{}).(cache.Caches); exists {
		asyncContext = context.WithValue(asyncContext, entity.CacheContext{}, cacheCtx)
	}
	if jwtCtx, exists := ctx.Value(JwtContext{}).(*entity.JWTClaimData); exists {
		asyncContext = context.WithValue(asyncContext, JwtContext{}, jwtCtx)
	}

	if traceId, exists := ctx.Value(common.TraceIdKeyContextStr).(string); exists {
		asyncContext = context.WithValue(asyncContext, common.TraceIdKeyContextStr, traceId)
	}

	return asyncContext
}
