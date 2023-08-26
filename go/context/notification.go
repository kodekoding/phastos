package context

import (
	"context"
	"net/http"

	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/kodekoding/phastos/v2/go/notifications"
)

func SetNotif(req *http.Request, slack notifications.Platforms) {
	ctx := context.WithValue(req.Context(), entity.NotifPlatformContext{}, slack)
	*req = *req.WithContext(ctx)
}

func GetNotif(ctx context.Context) notifications.Platforms {
	notifPlatform, valid := ctx.Value(entity.NotifPlatformContext{}).(notifications.Platforms)
	if !valid {
		return nil
	}
	return notifPlatform
}

func SetNotifDestination(req *http.Request, destination string) {
	ctx := context.WithValue(req.Context(), entity.NotifDestinationContext{}, destination)
	*req = *req.WithContext(ctx)
}

func GetNotifDestination(ctx context.Context) string {
	notifPlatform, valid := ctx.Value(entity.NotifDestinationContext{}).(string)
	if !valid {
		return ""
	}
	return notifPlatform
}
