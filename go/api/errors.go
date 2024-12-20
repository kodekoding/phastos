package api

import (
	"net/http"
)

type HttpError struct {
	Message    string      `json:"message"`
	Code       string      `json:"code"`
	Status     int         `json:"-"`
	TraceId    string      `json:"trace_id"`
	Data       interface{} `json:"data,omitempty"`
	CallerPath string      `json:"caller_path,omitempty"`
}

type ErrorOption func(*HttpError)

func (e *HttpError) Write(w http.ResponseWriter) {
	w.WriteHeader(e.Status)
	WriteJson(w, e)
}

func (e *HttpError) Error() string {
	return e.Message
}

func WithErrorCode(code string) ErrorOption {
	return func(e *HttpError) {
		e.Code = code
	}
}

func WithErrorStatus(status int) ErrorOption {
	return func(e *HttpError) {
		e.Status = status
	}
}

func WithErrorMessage(message string) ErrorOption {
	return func(e *HttpError) {
		e.Message = message
	}
}

func WithErrorData(data interface{}) ErrorOption {
	return func(e *HttpError) {
		e.Data = data
	}
}

func WithTraceId(traceId string) ErrorOption {
	return func(e *HttpError) {
		e.TraceId = traceId
	}
}

func WithErrorCallerPath(callerPath string) ErrorOption {
	return func(e *HttpError) {
		e.CallerPath = callerPath
	}
}

func NewErr(opts ...ErrorOption) *HttpError {
	httpErr := &HttpError{
		Status:  http.StatusInternalServerError,
		Code:    "SERVER_ERROR",
		Message: "an error occured",
	}

	for _, opt := range opts {
		opt(httpErr)
	}

	return httpErr
}

func InternalServerError(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  500,
	}
}

func UnprocessableEntity(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  422,
	}
}

func NotFound(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  404,
	}
}

func MethodNotAllowed(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  405,
	}
}

func Unauthorized(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  401,
	}
}

func Forbidden(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  403,
	}
}

func BadRequest(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  400,
	}
}

func TooManyRequest(message, code string) *HttpError {
	return &HttpError{
		Code:    code,
		Message: message,
		Status:  429,
	}
}
