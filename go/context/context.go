package context

import (
	"context"
	"net/http"

	"github.com/kodekoding/phastos/go/notifications"
)

type (
	notifPlatformContext struct{}
)

func SetNotif(req *http.Request, slack notifications.Platforms) {
	ctx := context.WithValue(req.Context(), notifPlatformContext{}, slack)
	*req = *req.WithContext(ctx)
}

func GetNotif(ctx context.Context) notifications.Platforms {
	notifPlatform, valid := ctx.Value(notifPlatformContext{}).(notifications.Platforms)
	if !valid {
		return nil
	}
	return notifPlatform
}
