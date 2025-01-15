package api

import (
	"encoding/json"
	"net/http"
	"runtime/debug"

	sgw "github.com/ashwanthkumar/slack-go-webhook"
	"github.com/rs/zerolog/log"

	context2 "github.com/kodekoding/phastos/v2/go/context"
)

func panicRecover(r *http.Request, traceId string, uniqueKey ...string) {
	if err := recover(); err != nil {
		stackTrace := string(debug.Stack())

		marshalErr, _ := json.Marshal(err)
		notifDetail := new(sgw.Attachment)
		if uniqueKey != nil && len(uniqueKey) > 0 {
			uniqueKeyReq := uniqueKey[0]
			notifDetail.AddField(sgw.Field{
				Title: "Unique Key Request",
				Value: uniqueKeyReq,
				Short: true,
			})
		}
		notifDetail.AddField(sgw.Field{
			Title: "IP",
			Value: r.RemoteAddr,
			Short: true,
		}).AddField(sgw.Field{
			Title: "Request Method",
			Value: r.Method,
			Short: true,
		}).AddField(sgw.Field{
			Title: "Request URI",
			Value: r.RequestURI,
			Short: true,
		}).AddField(sgw.Field{
			Title: "Host",
			Value: r.Host,
			Short: true,
		}).AddField(sgw.Field{
			Title: "Path",
			Value: r.URL.Path,
			Short: true,
		}).AddField(sgw.Field{
			Title: "StackTrace",
			Value: stackTrace,
		}).AddField(sgw.Field{
			Title: "Error",
			Value: string(marshalErr),
		}).AddField(sgw.Field{
			Title: "App Version",
			Value: appVersion,
			Short: true,
		})

		asyncCtx := context2.CreateAsyncContext(r.Context())
		go func() {
			notif := context2.GetNotif(asyncCtx)
			allNotifPlatform := notif.GetAllPlatform()
			for _, service := range allNotifPlatform {
				if service.IsActive() {
					service.SetTraceId(traceId)

					if err := service.Send(asyncCtx, "your API is panic !!", notifDetail); err != nil {
						log.Error().Msgf("error when send %s notifications: %s", service.Type(), err.Error())
					}
				}

			}
		}()
		log.Error().Any("data", notifDetail).Msg("got panic at handler")
	}
}
