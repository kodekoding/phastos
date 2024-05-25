package cache

import (
	"context"
	"github.com/kodekoding/phastos/v2/go/entity"
)

func GetCacheFromContext(ctx context.Context) Caches {
	cache, valid := ctx.Value(entity.CacheContext{}).(Caches)
	if !valid {
		return nil
	}
	return cache
}
