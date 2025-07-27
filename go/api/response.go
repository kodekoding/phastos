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
	plog "github.com/kodekoding/phastos/v2/go/log"
)

type ResponseRecorder struct {
	http.ResponseWriter
	StatusCode int
}

func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{ResponseWriter: w, StatusCode: http.StatusOK}
}

// WriteHeader overrides the default WriteHeader to store the status code.
func (r *ResponseRecorder) WriteHeader(status int) {
	r.StatusCode = status
	r.ResponseWriter.WriteHeader(status)
}

type Response struct {
	Message          string `json:"message,omitempty"`
	Data             any    `json:"data,omitempty"`
	Err              error  `json:"error,omitempty"`
	TraceId          string `json:"trace_id"`
	isPaginationData bool
	statusCode       int
	InternalError    *HttpError                 `json:"-"`
	MetaData         *database.ResponseMetaData `json:"metadata,omitempty"`
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

func (resp *Response) SetData(data any, isPaginate ...bool) *Response {
	resp.Data = data
	if selectResponseData, valid := data.(*database.SelectResponse); valid {
		if selectResponseData.ResponseMetaData != nil {
			resp.MetaData = selectResponseData.ResponseMetaData
		}
		resp.Data = selectResponseData.Data
	}

	if isPaginate != nil && len(isPaginate) > 0 {
		resp.isPaginationData = isPaginate[0]
	}
	return resp
}

func (resp *Response) Send(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	var b []byte

	var dataToMarshal any
	var responseStatus int
	if resp.Err != nil {
		if respErr, ok := resp.Err.(*HttpError); ok {
			responseStatus = respErr.Status
			dataToMarshal = respErr
		}
	} else {
		responseStatus = resp.statusCode
		bodyContentAvailable := false
		if resp.Data != nil {
			bodyContentAvailable = true
			dataToMarshal = resp.Data
			if resp.MetaData != nil && resp.isPaginationData {
				dataToMarshal = map[string]any{
					"data":     resp.Data,
					"metadata": resp.MetaData,
				}
			}
		}

		if resp.Message != "" {
			bodyContentAvailable = true

			dataToMarshal = map[string]string{
				"message": resp.Message,
			}
		}

		if !bodyContentAvailable {
			resp.statusCode = http.StatusNoContent
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
	log := plog.Get()
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
						bodyReq, _ := readAllContent(r.Body)
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
								Title: "Referer",
								Value: r.Header.Get("Referer"),
							}).AddField(
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
							log.Err(err).Msgf("error when sent %s notifications", notif.Type())
						}
					}
				}
			}
		}
	}
}
