package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	custerr "github.com/kodekoding/phastos/v2/go/error"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResponse_ReleaseResponse(t *testing.T) {
	resp := NewResponse()
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.statusCode)
	ReleaseResponse(resp)
}

func TestResponse_SetMessage(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	result := resp.SetMessage("hello")
	assert.Equal(t, resp, result)
	assert.Equal(t, "hello", resp.Message)
}

func TestResponse_SetStatusCode(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	result := resp.SetStatusCode(http.StatusCreated)
	assert.Equal(t, resp, result)
	assert.Equal(t, http.StatusCreated, resp.statusCode)
}

func TestResponse_SetCustomHeader(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	result := resp.SetCustomHeader("X-Custom", "value")
	assert.Equal(t, resp, result)
	assert.Equal(t, "value", resp.customHeader["X-Custom"])
}

func TestResponse_SetFileDownload(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	data := []byte("file content")
	result := resp.SetFileDownload(data, "report.csv", "text/csv")
	assert.Equal(t, resp, result)
	assert.Equal(t, data, resp.fileData)
	assert.Equal(t, "report.csv", resp.fileDownloadName)
	assert.Equal(t, "text/csv", resp.fileContentType)
}

func TestResponse_SetData(t *testing.T) {
	t.Run("plain data", func(t *testing.T) {
		resp := NewResponse()
		defer ReleaseResponse(resp)
		result := resp.SetData(map[string]string{"key": "val"})
		assert.Equal(t, resp, result)
		assert.NotNil(t, resp.Data)
	})

	t.Run("with pagination", func(t *testing.T) {
		resp := NewResponse()
		defer ReleaseResponse(resp)
		result := resp.SetData(map[string]string{"key": "val"}, true)
		assert.Equal(t, resp, result)
		assert.True(t, resp.isPaginationData)
	})
}

func TestResponse_SetHTTPError(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	httpErr := BadRequest("bad", "BAD")
	result := resp.SetHTTPError(httpErr)
	assert.Equal(t, resp, result)
	assert.Equal(t, httpErr, resp.Err)
}

func TestResponse_SetError_NonHttpError(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	result := resp.SetError(assert.AnError)
	assert.Equal(t, resp, result)
	assert.NotNil(t, resp.InternalError)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", resp.InternalError.Code)
}

func TestResponse_SetError_HttpError(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	badErr := BadRequest("bad request", "BAD")
	result := resp.SetError(badErr)
	assert.Equal(t, resp, result)
	assert.Equal(t, badErr, resp.InternalError)
}

func TestResponse_SetError_HttpError500(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	srvErr := InternalServerError("internal", "INTERNAL")
	result := resp.SetError(srvErr)
	assert.Equal(t, resp, result)
	assert.NotNil(t, resp.InternalError)
}

func TestResponse_Send_MessageOnly(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	resp.SetMessage("success")

	w := httptest.NewRecorder()
	resp.Send(w)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "success", body["message"])
}

func TestResponse_Send_DataOnly(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	resp.SetData(map[string]string{"name": "test"})

	w := httptest.NewRecorder()
	resp.Send(w)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestResponse_Send_WithHttpError(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	resp.SetError(Unauthorized("unauthorized", "UNAUTH"))

	w := httptest.NewRecorder()
	resp.Send(w)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestResponse_Send_FileDownload(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	resp.SetFileDownload([]byte("data"), "file.txt", "text/plain")

	w := httptest.NewRecorder()
	resp.Send(w)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "file.txt")
}

func TestResponse_Send_EmptyBody(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)

	w := httptest.NewRecorder()
	resp.Send(w)

	// When no data/message/error, responseStatus is captured from resp.statusCode (200)
	// before bodyContentAvailable check modifies resp.statusCode
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestResponse_Send_WithCustomHeader(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	resp.SetMessage("ok").SetCustomHeader("X-Test", "value")

	w := httptest.NewRecorder()
	resp.Send(w)

	assert.Equal(t, "value", w.Header().Get("X-Test"))
}

func TestResponse_Send_WithTraceId(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)
	resp.SetError(BadRequest("bad", "BAD"))
	resp.TraceId = "trace-123"

	w := httptest.NewRecorder()
	resp.Send(w)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResponseRecorder(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := NewResponseRecorder(inner)

	assert.Equal(t, http.StatusOK, rec.StatusCode)

	rec.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, rec.StatusCode)
}

func TestWriteJson(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJson(w, map[string]string{"key": "value"})

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "value", body["key"])
}

func TestHttpError_Write(t *testing.T) {
	w := httptest.NewRecorder()
	err := NotFound("not found", "NOT_FOUND")
	err.Write(w)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestResponse_SetError_ConflictConstraintError(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)

	reqErr := custerr.New(assert.AnError).SetCode(http.StatusConflict)
	err := errors.Wrap(reqErr, "repository.Insert")
	result := resp.SetError(err)

	assert.Equal(t, resp, result)
	assert.Equal(t, "DATA_CONFLICT", resp.InternalError.Code)
	assert.Equal(t, http.StatusConflict, resp.InternalError.Status)
}

func TestResponse_SetError_UnprocessableConstraintError(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)

	reqErr := custerr.New(assert.AnError).SetCode(http.StatusUnprocessableEntity)
	err := errors.Wrap(reqErr, "repository.Insert")
	result := resp.SetError(err)

	assert.Equal(t, resp, result)
	assert.Equal(t, "CONSTRAINT_VIOLATION", resp.InternalError.Code)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.InternalError.Status)
}

func TestResponse_SetError_ConflictConstraintError_WithRegistryMatch(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)

	reqErr := custerr.New(assert.AnError).SetCode(http.StatusConflict)
	reqErr.AppendData("constraint", "users_email_key")
	reqErr.AppendData("sql_state", "23505")
	err := errors.Wrap(reqErr, "repository.Insert")
	result := resp.SetError(err)

	assert.Equal(t, resp, result)
	assert.Equal(t, "EMAIL_ALREADY_EXISTS", resp.InternalError.Code)
	assert.Equal(t, "Email already registered", resp.InternalError.Message)
	assert.Equal(t, http.StatusConflict, resp.InternalError.Status)
}

func TestResponse_SetError_ConflictConstraintError_WithRegistryMatch_NIK(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)

	reqErr := custerr.New(assert.AnError).SetCode(http.StatusConflict)
	reqErr.AppendData("constraint", "users_ktp_no_key")
	reqErr.AppendData("sql_state", "23505")
	err := errors.Wrap(reqErr, "repository.Insert")
	result := resp.SetError(err)

	assert.Equal(t, resp, result)
	assert.Equal(t, "NIK_ALREADY_EXISTS", resp.InternalError.Code)
	assert.Equal(t, "NIK already exists", resp.InternalError.Message)
	assert.Equal(t, http.StatusConflict, resp.InternalError.Status)
}

func TestResponse_SetError_NonConstraintRequestError(t *testing.T) {
	resp := NewResponse()
	defer ReleaseResponse(resp)

	// RequestError with code=500 should still be treated as generic internal error
	reqErr := custerr.New(assert.AnError).SetCode(http.StatusInternalServerError)
	err := errors.Wrap(reqErr, "repository.Insert")
	result := resp.SetError(err)

	assert.Equal(t, resp, result)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", resp.InternalError.Code)
}
