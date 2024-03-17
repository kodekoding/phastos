package api

import (
	"bytes"
	contextpkg "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	sgw "github.com/ashwanthkumar/slack-go-webhook"
	"github.com/pkg/errors"

	"github.com/kodekoding/phastos/v2/go/context"
	"github.com/kodekoding/phastos/v2/go/database"
	"github.com/kodekoding/phastos/v2/go/env"
	"github.com/kodekoding/phastos/v2/go/log"
)

type Response struct {
	Message       string      `json:"message,omitempty"`
	Data          interface{} `json:"data,omitempty"`
	Err           error       `json:"error,omitempty"`
	statusCode    int
	InternalError *HttpError                 `json:"-"`
	MetaData      *database.ResponseMetaData `json:"metadata,omitempty"`
}

func NewResponse() *Response {
	return &Response{
		statusCode: http.StatusOK,
	}
}

func (resp *Response) SetMessage(msg string) *Response {
	resp.Message = msg
	return resp
}

func (resp *Response) SetStatusCode(statusCode int) *Response {
	resp.statusCode = statusCode
	return resp
}

func (resp *Response) SetData(data interface{}) *Response {
	resp.Data = data
	if selectResponseData, valid := data.(*database.SelectResponse); valid {
		if selectResponseData.ResponseMetaData != nil {
			resp.MetaData = selectResponseData.ResponseMetaData
		}
		resp.Data = selectResponseData.Data
	}
	return resp
}

func (resp *Response) Send(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	var b []byte

	var dataToMarshal interface{}
	var responseStatus int
	if resp.Err != nil {
		if respErr, ok := resp.Err.(*HttpError); ok {
			responseStatus = respErr.Status
			dataToMarshal = respErr
		}
	} else {
		responseStatus = resp.statusCode
		if resp.Data != nil {
			dataToMarshal = resp.Data
		} else if resp.Message != "" {
			dataToMarshal = map[string]string{
				"message": resp.Message,
			}
		}
	}

	w.WriteHeader(responseStatus)
	b, _ = json.Marshal(dataToMarshal)
	_, _ = w.Write(b)
}

func (resp *Response) SetError(err error) *Response {
	resp.Err = err
	if causeErr, isHttpErr := errors.Cause(err).(*HttpError); !isHttpErr {
		// if not httpError then create new httpError for internal error and sent alert to notification platform
		resp.InternalError = NewErr(WithErrorCode("INTERNAL_SERVER_ERROR"), WithErrorMessage(err.Error()))
		resp.Err = errors.New("Internal Server Error")
	} else {
		resp.InternalError = causeErr
		if causeErr.Status == http.StatusInternalServerError {
			resp.Err = errors.New("Internal Server Error")
		}
	}

	return resp
}

func (resp *Response) SetHTTPError(err *HttpError) *Response {
	resp.Err = err
	return resp
}

func (resp *Response) SentNotif(ctx contextpkg.Context, err *HttpError, r *http.Request, traceId string) {
	if err == nil {
		return
	}
	getNotifContext := context.GetNotif(ctx)
	if getNotifContext != nil {
		for _, notif := range getNotifContext.GetAllPlatform() {
			if notif.IsActive() {
				if notif.Type() == "slack" {

					notif.SetTraceId(traceId)
					if err.Status == 500 {
						bodyReq, _ := io.ReadAll(r.Body)
						r.Body = io.NopCloser(bytes.NewBuffer(bodyReq))
						if string(bodyReq) == "" || r.Method == http.MethodGet {
							bodyReq, _ = json.Marshal(r.URL.Query())
						}
						slackAttachment := new(sgw.Attachment)
						color := "#ff0e0a"
						slackAttachment.Color = &color
						slackAttachment.AddField(
							sgw.Field{
								Short: true,
								Title: "Error Status",
								Value: fmt.Sprintf("%d", err.Status),
							}).AddField(
							sgw.Field{
								Short: true,
								Title: "Error Code",
								Value: err.Code,
							}).AddField(
							sgw.Field{
								Title: "Body Request",
								Value: string(bodyReq),
							}).AddField(
							sgw.Field{
								Title: "Description",
								Value: err.Message,
							}).AddField(
							sgw.Field{
								Short: true,
								Title: "Route Path",
								Value: fmt.Sprintf("%s: %s", r.Method, r.URL.Path),
							}).AddField(
							sgw.Field{
								Short: true,
								Title: "Environment",
								Value: env.ServiceEnv(),
							})
						if err.Data != nil {
							errData, _ := json.Marshal(err.Data)
							slackAttachment.AddField(sgw.Field{
								Title: "Error Data",
								Value: string(errData),
							})
						}

						if err.CallerPath != "" {
							slackAttachment.AddField(sgw.Field{
								Title: "Caller Function / Context",
								Value: err.CallerPath,
							})
						}

						notifTitle := fmt.Sprintf("Error Processing Request on %s", env.ServiceEnv())
						if err := notif.Send(ctx, notifTitle, slackAttachment); err != nil {
							log.Errorln("error when sent", notif.Type(), " notifications: ", err.Error())
						}
					}
				}
			}
		}
	}
}
