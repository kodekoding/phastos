package response

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	sgw "github.com/ashwanthkumar/slack-go-webhook"
	"github.com/pkg/errors"

	ctxlib "github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/env"
	cutomerr "github.com/kodekoding/phastos/v2/go/error"
	"github.com/kodekoding/phastos/v2/go/helper"
	"github.com/rs/zerolog/log"
)

type JSON struct {
	Code        int         `json:"code"`
	Message     string      `json:"message"`
	MessageDesc string      `json:"description,omitempty"`
	Error       error       `json:"-"`
	Data        interface{} `json:"data,omitempty"`
	Latency     float64     `json:"latency"`
	TraceId     string      `json:"trace_id,omitempty"`
	ExportFile  *ExportFile `json:"-"`
}

type ExportFile struct {
	Name    string        `json:"name"`
	Content *bytes.Buffer `json:"-"`
}

var mapDefaultErrMsg = map[int]string{
	500: "Internal Server Error",
	400: "Bad Request",
	403: "Forbidden Access",
	401: "Unauthorized",
}

func NewJSON() *JSON {
	return &JSON{}
}

func (jr *JSON) SetMessage(msg string) *JSON {
	jr.Message = msg
	return jr
}

func (jr *JSON) SetCode(code int) *JSON {
	jr.Code = code
	return jr
}

func (jr *JSON) SetData(data interface{}) *JSON {
	jr.Data = data
	jr.Code = 200
	return jr
}

func (jr *JSON) SetError(errCode int, err error) *JSON {
	jr.Code = errCode
	jr.Error = err
	return jr
}

func (jr *JSON) Export(fileName string, content *bytes.Buffer) *JSON {
	//Create New JSON to overriding everything
	*jr = JSON{
		ExportFile: &ExportFile{
			Name:    fileName,
			Content: content,
		},
	}

	return jr
}

func (jr *JSON) Success(data interface{}) *JSON {
	jr.Data = data
	jr.Message = "SUCCESS"
	jr.Code = 200
	return jr
}

func (jr *JSON) BadRequest(err error) *JSON {
	jr.Message = "Bad Request"
	jr.MessageDesc = err.Error()
	jr.Code = 400
	return jr
}

func (jr *JSON) ForbiddenResource(err error) *JSON {
	jr.Message = "Forbiden Resource"
	jr.MessageDesc = "you're not allowed to access this feature"
	jr.Code = 403
	jr.Error = err
	return jr
}

func (jr *JSON) ExpiredToken(err error) *JSON {
	jr.Message = "Token Expired"
	jr.MessageDesc = err.Error()
	jr.Code = 403
	jr.Error = err
	return jr
}

func (jr *JSON) Unauthorized(err error) *JSON {
	jr.Message = "Unauthorized"
	jr.MessageDesc = "you're not authorized to access this feature"
	jr.Code = 401
	jr.Error = err
	return jr
}

func (jr *JSON) InternalServerError(err error) *JSON {
	jr.Message = "Internal Server Error"
	jr.MessageDesc = "Please contact your administrator for detailed error message"
	jr.Code = 500
	jr.Error = err
	return jr
}

func (jr *JSON) ErrorChecking(r *http.Request) bool {
	if jr.Error != nil {
		// error occurred
		var usingErr error
		causeErr := errors.Cause(jr.Error)
		customErr, valid := causeErr.(*cutomerr.RequestError)
		var optionalData string
		if !valid {
			usingErr = jr.Error
		} else {
			statusCode := customErr.GetCode()
			if statusCode != http.StatusInternalServerError {
				jr.Code = statusCode
				jr.Message = customErr.Error()
			}
			custErrMsgDesc := customErr.GetMessage()
			if custErrMsgDesc != "" {
				jr.MessageDesc = custErrMsgDesc
			}
			errData, err := json.Marshal(customErr.GetData())
			if err != nil {
				log.Err(err).Msg("error when marshal optional data:")
			} else {
				optionalData = string(errData)
			}
			usingErr = customErr
		}
		ctx := r.Context()
		traceId := helper.GenerateUUIDV4()
		jr.TraceId = traceId

		errMsg := fmt.Sprintf(`[%s] code %d (%s) - %s: %s`, traceId, jr.Code, env.ServiceEnv(), r.URL.Path, usingErr.Error())
		notifMsg := fmt.Sprintf(`%s
			%s`, errMsg, optionalData)
		go jr.sendNotif(ctx, r, optionalData, notifMsg, usingErr.Error())
		// print log
		log.Error().Msg(errMsg)
		return true
	} else if jr.Code == 200 && r.Method != http.MethodGet {
		// if the request is CUD (Create Update Delete) and SUCCESS, then log to info file
		bodyReq, _ := ioutil.ReadAll(r.Body)
		log.Info().Msgf("%v", map[string]interface{}{
			"body":   string(bodyReq),
			"header": r.Header,
			"params": r.URL.RawQuery,
			"path":   r.URL.Path,
		})
	}

	return false
}

func (jr *JSON) sendNotif(ctx context.Context, r *http.Request, notifMsg, optionalData, errStr string) {
	notif := ctxlib.GetNotif(ctx)
	if notif != nil {
		allNotifPlatform := notif.GetAllPlatform()
		var attachment interface{}
		for _, platform := range allNotifPlatform {
			if platform.IsActive() {
				attachment = nil

				notifMsg = fmt.Sprintf(`
							Hi All there's an error: 
							%s
						`, notifMsg)
				if platform.Type() == "slack" {
					channelDestination := ctxlib.GetNotifDestination(ctx)
					if channelDestination != "" && env.IsProduction() && jr.Code == 500 {
						// if the code is 500, then send
						platform.SetDestination(channelDestination)
					}
					platform.SetTraceId(jr.TraceId)
					bodyReq, _ := io.ReadAll(r.Body)
					r.Body = io.NopCloser(bytes.NewBuffer(bodyReq))
					slackAttachment := new(sgw.Attachment)
					color := "#ff0e0a"
					slackAttachment.Color = &color
					slackAttachment.AddField(
						sgw.Field{
							Short: true,
							Title: "Error Code",
							Value: fmt.Sprintf("%d", jr.Code),
						}).AddField(
						sgw.Field{
							Title: "Body Request",
							Value: string(bodyReq),
						}).AddField(
						sgw.Field{
							Title: "Referer",
							Value: r.Referer(),
						}).AddField(
						sgw.Field{
							Title: "Description",
							Value: errStr,
						}).AddField(
						sgw.Field{
							Short: true,
							Title: "Route Path",
							Value: fmt.Sprintf("%s - %s", r.Method, r.URL.Path),
						}).AddField(
						sgw.Field{
							Short: true,
							Title: "Environment",
							Value: env.ServiceEnv(),
						})

					if optionalData != "" && optionalData != "{}" {
						slackAttachment.AddField(sgw.Field{
							Title: "Optional Data",
							Value: optionalData,
						})
					}
					attachment = slackAttachment
					notifMsg = ""
				}
				if jr.Code == 500 {
					if err := platform.Send(ctx, notifMsg, attachment); err != nil {
						log.Err(err).Msgf("error when send %s notifications", platform.Type())
					}
				}
			}
		}
	}
}

func (jr *JSON) Send(w http.ResponseWriter) {
	/* #nosec */
	if jr.ExportFile != nil {
		jr.sendExport(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(jr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr := w.Write([]byte(`{"errors":["Internal Server Error"]}`))
		if writeErr != nil {
			log.Err(writeErr).Msg("Error when Send Response")
		}
	}

	if jr.Code == 0 {
		jr.Code = 500
	}
	w.WriteHeader(jr.Code)
	_, _ = w.Write(b)
	return
}

func (jr *JSON) sendExport(w http.ResponseWriter) {
	contentDisposition := fmt.Sprintf("attachment; filename=%s", jr.ExportFile.Name)
	w.Header().Set("Content-Disposition", contentDisposition)
	if _, err := io.Copy(w, jr.ExportFile.Content); err != nil {
		NewJSON().InternalServerError(err)
	}

	return
}
