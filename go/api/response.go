package api

import (
	"bytes"
	contextpkg "context"
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
	Message       string                     `json:"message,omitempty"`
	Data          interface{}                `json:"data,omitempty"`
	Err           error                      `json:"error,omitempty"`
	InternalError *HttpError                 `json:"-"`
	TraceId       string                     `json:"trace_id"`
	MetaData      *database.ResponseMetaData `json:"metadata,omitempty"`
}

func NewResponse() *Response {
	return &Response{}
}

func (resp *Response) SetMessage(msg string) *Response {
	resp.Message = msg
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

func (resp *Response) SetError(err error) *Response {
	if causeErr, isHttpErr := errors.Cause(err).(*HttpError); isHttpErr {
		resp.InternalError = NewErr(WithCode(causeErr.Code), WithMessage(causeErr.Message), WithStatus(causeErr.Status))
	}
	resp.Err = err

	return resp
}

func (resp *Response) SetHTTPError(err *HttpError) *Response {
	resp.Err = err
	return resp
}

func (resp *Response) SentNotif(ctx contextpkg.Context, err *HttpError, r *http.Request) {
	getNotifContext := context.GetNotif(ctx)
	if getNotifContext != nil {
		for _, notif := range getNotifContext.GetAllPlatform() {
			if notif.IsActive() {
				if notif.Type() == "slack" {

					notif.SetTraceId(resp.TraceId)
					if err.Status == 500 {
						bodyReq, _ := io.ReadAll(r.Body)
						r.Body = io.NopCloser(bytes.NewBuffer(bodyReq))
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
						if err := notif.Send(ctx, "Error Processing Request", slackAttachment); err != nil {
							log.Errorln("error when sent", notif.Type(), " notifications: ", err.Error())
						}
					}
				}
			}
		}
	}
}
