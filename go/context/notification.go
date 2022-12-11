package context

import (
	"context"
	"net/http"

	"github.com/kodekoding/phastos/go/notifications"
)

type (
	notifPlatformContext    struct{}
	notifDestinationContext struct{}
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

func SetNotifDestination(req *http.Request, destination string) {
	ctx := context.WithValue(req.Context(), notifDestinationContext{}, destination)
	*req = *req.WithContext(ctx)
}

func GetNotifDestination(ctx context.Context) string {
	notifPlatform, valid := ctx.Value(notifDestinationContext{}).(string)
	if !valid {
		return ""
	}
	return notifPlatform
}
