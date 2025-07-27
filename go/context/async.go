package context

import (
	"context"

	"github.com/kodekoding/phastos/v2/go/cache"
	"github.com/kodekoding/phastos/v2/go/entity"
	plog "github.com/kodekoding/phastos/v2/go/log"
	"github.com/kodekoding/phastos/v2/go/monitoring"
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

	if traceId, exists := ctx.Value("traceId").(string); exists {
		asyncContext = context.WithValue(asyncContext, "traceId", traceId)
	}

	// embed log context to async context if exists
	logCtx := plog.Ctx(ctx)
	if logCtx != nil {
		asyncContext = logCtx.WithContext(asyncContext)
	}

	// embed new relic context if exists
	newRelicContext := monitoring.BeginTrxFromContext(ctx)
	if newRelicContext != nil {
		asyncContext = monitoring.NewContext(asyncContext, newRelicContext.NewGoroutine())
	}

	return asyncContext
}
