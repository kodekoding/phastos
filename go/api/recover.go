package api

import (
	"encoding/json"
	"fmt"
	context2 "github.com/kodekoding/phastos/v2/go/context"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"runtime/debug"
)

func panicRecover(r *http.Request, traceId string) {
	if err := recover(); err != nil {
		stackTrace := string(debug.Stack())
		b, _ := io.ReadAll(r.Body)

		fields := map[string]interface{}{
			"Request.Body":  string(b),
			"Host":          r.Host,
			"RequestMethod": r.Method,
			"RequestURI":    r.RequestURI,
			"Error":         err,
			"Path":          r.URL.Path,
		}

		panicData := fields
		panicData["StackTrace"] = stackTrace
		panicData["IP"] = r.RemoteAddr

		panicData["StackError"] = err
		ctx := r.Context()
		byteData, _ := json.Marshal(panicData)
		notifMsg := fmt.Sprintf(`your API is panic:
			%s
			traceID: %s`, string(byteData), traceId)
		notif := context2.GetNotif(r.Context())
		allNotifPlatform := notif.GetAllPlatform()
		go func() {
			for _, service := range allNotifPlatform {
				if service.IsActive() {
					if err := service.Send(ctx, notifMsg, nil); err != nil {
						log.Error().Msgf("error when send %s notifications: %s", service.Type(), err.Error())
					}
				}

			}
		}()
		log.Error().Msgf("got panic at handler: %v", fields)
	}
}
