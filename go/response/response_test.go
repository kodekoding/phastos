package response

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kodekoding/phastos/v2/go/entity"
	cutomerr "github.com/kodekoding/phastos/v2/go/error"
	"github.com/kodekoding/phastos/v2/go/notifications"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failWriter is an http.ResponseWriter that always fails on Write.
type failWriter struct {
	header http.Header
}

func (f *failWriter) Header() http.Header {
	if f.header == nil {
		f.header = make(http.Header)
	}
	return f.header
}

func (f *failWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (f *failWriter) WriteHeader(statusCode int) {
}

func TestNewJSON(t *testing.T) {
	jr := NewJSON()
	assert.NotNil(t, jr)
	assert.Equal(t, 0, jr.Code)
	assert.Equal(t, "", jr.Message)
}

func TestSetMessage(t *testing.T) {
	jr := NewJSON().SetMessage("test message")
	assert.Equal(t, "test message", jr.Message)
}

func TestSetCode(t *testing.T) {
	jr := NewJSON().SetCode(201)
	assert.Equal(t, 201, jr.Code)
}

func TestSetData(t *testing.T) {
	data := map[string]string{"key": "value"}
	jr := NewJSON().SetData(data)
	assert.Equal(t, 200, jr.Code)
	assert.Equal(t, data, jr.Data)
}

func TestSetError(t *testing.T) {
	jr := NewJSON().SetError(500, assert.AnError)
	assert.Equal(t, 500, jr.Code)
	assert.Equal(t, assert.AnError, jr.Error)
}

func TestSuccess(t *testing.T) {
	data := map[string]string{"result": "ok"}
	jr := NewJSON().Success(data)
	assert.Equal(t, 200, jr.Code)
	assert.Equal(t, "SUCCESS", jr.Message)
	assert.Equal(t, data, jr.Data)
}

func TestBadRequest(t *testing.T) {
	jr := NewJSON().BadRequest(assert.AnError)
	assert.Equal(t, 400, jr.Code)
	assert.Equal(t, "Bad Request", jr.Message)
	assert.Equal(t, assert.AnError.Error(), jr.MessageDesc)
}

func TestForbiddenResource(t *testing.T) {
	jr := NewJSON().ForbiddenResource(assert.AnError)
	assert.Equal(t, 403, jr.Code)
	assert.Equal(t, "Forbiden Resource", jr.Message)
	assert.Equal(t, assert.AnError, jr.Error)
}

func TestExpiredToken(t *testing.T) {
	jr := NewJSON().ExpiredToken(assert.AnError)
	assert.Equal(t, 403, jr.Code)
	assert.Equal(t, "Token Expired", jr.Message)
	assert.Equal(t, assert.AnError, jr.Error)
}

func TestUnauthorized(t *testing.T) {
	jr := NewJSON().Unauthorized(assert.AnError)
	assert.Equal(t, 401, jr.Code)
	assert.Equal(t, "Unauthorized", jr.Message)
	assert.Equal(t, assert.AnError, jr.Error)
}

func TestInternalServerError(t *testing.T) {
	jr := NewJSON().InternalServerError(assert.AnError)
	assert.Equal(t, 500, jr.Code)
	assert.Equal(t, "Internal Server Error", jr.Message)
	assert.Equal(t, assert.AnError, jr.Error)
}

func TestExport(t *testing.T) {
	buf := bytes.NewBufferString("file content")
	jr := NewJSON().Export("test.csv", buf)
	assert.NotNil(t, jr.ExportFile)
	assert.Equal(t, "test.csv", jr.ExportFile.Name)
	assert.Equal(t, "file content", jr.ExportFile.Content.String())
}

func TestSendSuccess(t *testing.T) {
	jr := NewJSON().Success(map[string]string{"hello": "world"})
	rec := httptest.NewRecorder()
	jr.Send(rec)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var result JSON
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, 200, result.Code)
	assert.Equal(t, "SUCCESS", result.Message)
}

func TestSendBadRequest(t *testing.T) {
	jr := NewJSON().BadRequest(assert.AnError)
	rec := httptest.NewRecorder()
	jr.Send(rec)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSendZeroCodeDefaultsTo500(t *testing.T) {
	jr := NewJSON()
	rec := httptest.NewRecorder()
	jr.Send(rec)
	// Code 0 defaults to 500 in Send
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestSendExportFile(t *testing.T) {
	buf := bytes.NewBufferString("hello export")
	jr := NewJSON().Export("report.csv", buf)
	rec := httptest.NewRecorder()
	jr.Send(rec)

	assert.Equal(t, "attachment; filename=report.csv", rec.Header().Get("Content-Disposition"))
}

func TestJSONFieldOmission(t *testing.T) {
	jr := NewJSON()
	jr.Code = 200
	jr.Message = "OK"

	b, err := json.Marshal(jr)
	assert.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(b, &result)
	assert.NoError(t, err)

	// Data, Latency, and TraceId should be omitted when zero
	_, hasData := result["data"]
	assert.False(t, hasData)

	// Error should be omitted (json:"-")
	_, hasError := result["Error"]
	assert.False(t, hasError)
}

func TestErrorCheckingNoError(t *testing.T) {
	jr := NewJSON()
	jr.Code = 200
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	result := jr.ErrorChecking(req)
	assert.False(t, result)
}

func TestErrorCheckingWithError(t *testing.T) {
	jr := NewJSON()
	jr.Error = assert.AnError
	jr.Code = 500
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	result := jr.ErrorChecking(req)
	assert.True(t, result)
	assert.NotEmpty(t, jr.TraceId)
}

// --- Mock implementations for notification platform ---

type mockAction struct {
	isActive   bool
	actionType string
	sendCalled bool
	sendErr    error
	destination interface{}
	traceId     string
}

func (m *mockAction) Send(ctx context.Context, text string, attachment interface{}) error {
	m.sendCalled = true
	return m.sendErr
}

func (m *mockAction) IsActive() bool {
	return m.isActive
}

func (m *mockAction) Type() string {
	return m.actionType
}

func (m *mockAction) SetTraceId(traceId string) {
	m.traceId = traceId
}

func (m *mockAction) SetDestination(destination interface{}) {
	m.destination = destination
}

// --- Tests for ErrorChecking with custom RequestError ---

func TestErrorCheckingWithCustomRequestError(t *testing.T) {
	customErr := cutomerr.New(assert.AnError).SetCode(400).SetMessage("custom bad request")

	jr := NewJSON()
	jr.Error = customErr
	jr.Code = 400
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	result := jr.ErrorChecking(req)
	assert.True(t, result)
	assert.Equal(t, 400, jr.Code)
	assert.NotEmpty(t, jr.TraceId)
}

func TestErrorCheckingWithCustomRequestError500(t *testing.T) {
	customErr := cutomerr.New(assert.AnError).SetCode(500).SetMessage("internal error data")

	jr := NewJSON()
	jr.Error = customErr
	jr.Code = 500
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	result := jr.ErrorChecking(req)
	assert.True(t, result)
	assert.Equal(t, 500, jr.Code)
}

func TestErrorCheckingWithCustomRequestErrorEmptyMessage(t *testing.T) {
	customErr := cutomerr.New(assert.AnError).SetCode(401)

	jr := NewJSON()
	jr.Error = customErr
	jr.Code = 401
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	result := jr.ErrorChecking(req)
	assert.True(t, result)
	assert.Equal(t, 401, jr.Code)
}

// --- Tests for sendNotif ---

type mockActionForSendNotif struct {
	isActive   bool
	actionType string
	sendErr    error
	sendCalled bool
	dest       interface{}
	tid        string
}

func (m *mockActionForSendNotif) Send(ctx context.Context, text string, attachment interface{}) error {
	m.sendCalled = true
	return m.sendErr
}

func (m *mockActionForSendNotif) IsActive() bool {
	return m.isActive
}

func (m *mockActionForSendNotif) Type() string {
	return m.actionType
}

func (m *mockActionForSendNotif) SetTraceId(traceId string) {
	m.tid = traceId
}

func (m *mockActionForSendNotif) SetDestination(destination interface{}) {
	m.dest = destination
}

// mockPlatformsForSendNotif implements notifications.Platforms so that
// context.GetNotif can successfully type-assert it.
type mockPlatformsForSendNotif struct {
	actions []notifications.Action
}

func (mp *mockPlatformsForSendNotif) GetAllPlatform() []notifications.Action {
	return mp.actions
}

func (mp *mockPlatformsForSendNotif) Telegram() notifications.Action { return nil }
func (mp *mockPlatformsForSendNotif) Slack() notifications.Action    { return nil }
func (mp *mockPlatformsForSendNotif) FCM() notifications.Action      { return nil }
func (mp *mockPlatformsForSendNotif) WrapToHandler(next http.Handler) http.Handler { return next }
func (mp *mockPlatformsForSendNotif) WrapToContext(ctx context.Context) context.Context { return ctx }

func TestSendNotifNilPlatform(t *testing.T) {
	jr := NewJSON()
	jr.Code = 500
	jr.TraceId = "test-trace-id"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := req.Context()
	// No notification platform set, should not panic
	jr.sendNotif(ctx, req, "test message", "optional data", "error string")
}

func TestSendNotifInactivePlatform(t *testing.T) {
	mockAction := &mockActionForSendNotif{isActive: false, actionType: "slack"}
	mockPlatform := mockPlatformsForSendNotif{actions: []notifications.Action{mockAction}}

	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, &mockPlatform)

	jr := NewJSON()
	jr.Code = 500
	jr.TraceId = "test-trace-id"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)

	jr.sendNotif(ctx, req, "test message", "{}", "error string")
	assert.False(t, mockAction.sendCalled)
}

func TestSendNotifActiveSlackPlatform(t *testing.T) {
	mockAction := &mockActionForSendNotif{isActive: true, actionType: "slack", sendErr: nil}
	mockPlatform := mockPlatformsForSendNotif{actions: []notifications.Action{mockAction}}

	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, &mockPlatform)

	jr := NewJSON()
	jr.Code = 500
	jr.TraceId = "test-trace-id"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)

	jr.sendNotif(ctx, req, "test message", "{}", "error string")
	assert.True(t, mockAction.sendCalled)
	assert.Equal(t, "test-trace-id", mockAction.tid)
}

func TestSendNotifActiveSlackPlatformNon500(t *testing.T) {
	mockAction := &mockActionForSendNotif{isActive: true, actionType: "slack", sendErr: nil}
	mockPlatform := mockPlatformsForSendNotif{actions: []notifications.Action{mockAction}}

	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, &mockPlatform)

	jr := NewJSON()
	jr.Code = 400
	jr.TraceId = "test-trace-id"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)

	jr.sendNotif(ctx, req, "test message", "{}", "error string")
	assert.False(t, mockAction.sendCalled)
}

func TestSendNotifSendError(t *testing.T) {
	mockAction := &mockActionForSendNotif{isActive: true, actionType: "slack", sendErr: errors.New("send failed")}
	mockPlatform := mockPlatformsForSendNotif{actions: []notifications.Action{mockAction}}

	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, &mockPlatform)

	jr := NewJSON()
	jr.Code = 500
	jr.TraceId = "test-trace-id"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)

	jr.sendNotif(ctx, req, "test message", "{}", "error string")
	assert.True(t, mockAction.sendCalled)
}

func TestSendNotifNonSlackPlatform(t *testing.T) {
	mockAction := &mockActionForSendNotif{isActive: true, actionType: "telegram", sendErr: nil}
	mockPlatform := mockPlatformsForSendNotif{actions: []notifications.Action{mockAction}}

	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, &mockPlatform)

	jr := NewJSON()
	jr.Code = 500
	jr.TraceId = "test-trace-id"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)

	jr.sendNotif(ctx, req, "test message", "", "error string")
	assert.True(t, mockAction.sendCalled)
}

func TestSendNotifWithOptionalData(t *testing.T) {
	mockAction := &mockActionForSendNotif{isActive: true, actionType: "slack", sendErr: nil}
	mockPlatform := mockPlatformsForSendNotif{actions: []notifications.Action{mockAction}}

	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, &mockPlatform)

	jr := NewJSON()
	jr.Code = 500
	jr.TraceId = "test-trace-id"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)

	jr.sendNotif(ctx, req, "test message", `{"key":"value"}`, "error string")
	assert.True(t, mockAction.sendCalled)
}

func TestSendNotifWithEmptyOptionalData(t *testing.T) {
	mockAction := &mockActionForSendNotif{isActive: true, actionType: "slack", sendErr: nil}
	mockPlatform := mockPlatformsForSendNotif{actions: []notifications.Action{mockAction}}

	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, &mockPlatform)

	jr := NewJSON()
	jr.Code = 500
	jr.TraceId = "test-trace-id"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)

	jr.sendNotif(ctx, req, "test message", "", "error string")
	assert.True(t, mockAction.sendCalled)
}

func TestSendNotifWithEmptyJsonOptionalData(t *testing.T) {
	mockAction := &mockActionForSendNotif{isActive: true, actionType: "slack", sendErr: nil}
	mockPlatform := mockPlatformsForSendNotif{actions: []notifications.Action{mockAction}}

	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, &mockPlatform)

	jr := NewJSON()
	jr.Code = 500
	jr.TraceId = "test-trace-id"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)

	jr.sendNotif(ctx, req, "test message", "{}", "error string")
	assert.True(t, mockAction.sendCalled)
}

// --- Tests for ErrorChecking path where code == 200 && method != GET ---

func TestErrorCheckingCUDSuccessLogging(t *testing.T) {
	jr := NewJSON()
	jr.Code = 200
	jr.Error = nil
	req := httptest.NewRequest(http.MethodPost, "/api/test", bytes.NewBufferString(`{"key":"value"}`))
	req.Header.Set("Content-Type", "application/json")
	result := jr.ErrorChecking(req)
	assert.False(t, result)
}

func TestErrorCheckingCUDSuccessLoggingDelete(t *testing.T) {
	jr := NewJSON()
	jr.Code = 200
	jr.Error = nil
	req := httptest.NewRequest(http.MethodDelete, "/api/test", nil)
	result := jr.ErrorChecking(req)
	assert.False(t, result)
}

func TestErrorCheckingCUDSuccessLoggingPut(t *testing.T) {
	jr := NewJSON()
	jr.Code = 200
	jr.Error = nil
	req := httptest.NewRequest(http.MethodPut, "/api/test", bytes.NewBufferString(`{"updated":true}`))
	result := jr.ErrorChecking(req)
	assert.False(t, result)
}

// --- Tests for sendExport ---

func TestSendExport(t *testing.T) {
	buf := bytes.NewBufferString("exported content here")
	jr := NewJSON().Export("exported.csv", buf)

	rec := httptest.NewRecorder()
	jr.sendExport(rec)

	assert.Equal(t, "attachment; filename=exported.csv", rec.Header().Get("Content-Disposition"))
	assert.Equal(t, "exported content here", rec.Body.String())
}

func TestSendExportNilContent(t *testing.T) {
	jr := NewJSON()
	jr.ExportFile = &ExportFile{Name: "empty.csv", Content: bytes.NewBuffer(nil)}
	rec := httptest.NewRecorder()
	jr.sendExport(rec)
	assert.Equal(t, "attachment; filename=empty.csv", rec.Header().Get("Content-Disposition"))
	assert.Equal(t, "", rec.Body.String())
}

// --- Tests for Send method ---

func TestSendWithCode200(t *testing.T) {
	jr := NewJSON()
	jr.Code = 200
	jr.Message = "OK"
	rec := httptest.NewRecorder()
	jr.Send(rec)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSendWithCode404(t *testing.T) {
	jr := NewJSON()
	jr.Code = 404
	jr.Message = "Not Found"
	rec := httptest.NewRecorder()
	jr.Send(rec)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSendWithCode503(t *testing.T) {
	jr := NewJSON()
	jr.Code = 503
	jr.Message = "Service Unavailable"
	rec := httptest.NewRecorder()
	jr.Send(rec)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// --- Tests for JSON struct with Latency field ---

func TestJSONWithLatency(t *testing.T) {
	jr := NewJSON()
	jr.Code = 200
	jr.Message = "Success"
	jr.Latency = 123.45

	b, err := json.Marshal(jr)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(b, &result)
	require.NoError(t, err)
	val, ok := result["latency"]
	assert.True(t, ok)
	assert.Equal(t, 123.45, val)
}

// --- Tests for ErrorChecking with wrapped error ---

func TestErrorCheckingWithWrappedError(t *testing.T) {
	baseErr := errors.New("base error")
	wrappedErr := errors.Wrap(baseErr, "context")

	jr := NewJSON()
	jr.Error = wrappedErr
	jr.Code = 500
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	result := jr.ErrorChecking(req)
	assert.True(t, result)
	assert.NotEmpty(t, jr.TraceId)
}

// --- Tests for ErrorChecking with nil error but non-200 code ---

func TestErrorCheckingNilErrorNon200(t *testing.T) {
	jr := NewJSON()
	jr.Code = 201
	jr.Error = nil
	jr.Message = "Created"
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{}`))
	result := jr.ErrorChecking(req)
	assert.False(t, result)
}

// --- Tests for chained method calls ---

func TestChainedMethodCalls(t *testing.T) {
	jr := NewJSON().SetCode(200).SetMessage("OK").SetData(map[string]string{"key": "value"})
	assert.Equal(t, 200, jr.Code)
	assert.Equal(t, "OK", jr.Message)
	data := jr.Data.(map[string]string)
	assert.Equal(t, "value", data["key"])
}

// --- Test for sendNotif with body already consumed ---

func TestSendNotifBodyAlreadyConsumed(t *testing.T) {
	mockAction := &mockActionForSendNotif{isActive: true, actionType: "slack"}
	mockPlatform := mockPlatformsForSendNotif{actions: []notifications.Action{mockAction}}

	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, &mockPlatform)

	body := bytes.NewBufferString(`{"test":"data"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", body)

	jr := NewJSON()
	jr.Code = 500
	jr.TraceId = "test-trace-id"
	req = req.WithContext(ctx)

	jr.sendNotif(ctx, req, "first", "", "error1")
	jr.sendNotif(ctx, req, "second", "", "error2")

	assert.True(t, mockAction.sendCalled)
}

// --- Tests for ExportFile struct ---

func TestExportFileStruct(t *testing.T) {
	buf := bytes.NewBufferString("test content")
	ef := &ExportFile{Name: "test.txt", Content: buf}
	assert.Equal(t, "test.txt", ef.Name)
	assert.Equal(t, "test content", ef.Content.String())
}

// --- Test for sendNotif with no active platforms ---

func TestSendNotifWithNoActivePlatforms(t *testing.T) {
	mockAction := &mockActionForSendNotif{isActive: false, actionType: "slack"}
	mockPlatform := mockPlatformsForSendNotif{actions: []notifications.Action{mockAction}}

	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, &mockPlatform)

	jr := NewJSON()
	jr.Code = 500
	jr.TraceId = "test-trace-id"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(ctx)

	jr.sendNotif(ctx, req, "test message", "", "error string")
}

// --- Test for JSON marshal error in ErrorChecking (line 143) ---

func TestErrorCheckingWithUnmarshalableData(t *testing.T) {
	customErr := cutomerr.New(assert.AnError).SetCode(400).AppendData("ch", make(chan int))

	jr := NewJSON()
	jr.Error = customErr
	jr.Code = 400
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	result := jr.ErrorChecking(req)
	assert.True(t, result)
	assert.NotEmpty(t, jr.TraceId)
}

// --- Test for channelDestination branch in sendNotif (line 191) ---

func TestSendNotifSlackChannelDestination(t *testing.T) {
	t.Setenv("APPS_ENV", "production")

	mockAction := &mockActionForSendNotif{isActive: true, actionType: "slack", sendErr: nil}
	mockPlatform := mockPlatformsForSendNotif{actions: []notifications.Action{mockAction}}

	ctx := context.WithValue(context.Background(), entity.NotifPlatformContext{}, &mockPlatform)
	ctx = context.WithValue(ctx, entity.NotifDestinationContext{}, "C12345")

	jr := NewJSON()
	jr.Code = 500
	jr.TraceId = "test-trace-id"
	req := httptest.NewRequest(http.MethodGet, "/test", bytes.NewBufferString(`{"body":"test"}`))
	req = req.WithContext(ctx)

	jr.sendNotif(ctx, req, "test message", `{"key":"value"}`, "error string")
	assert.True(t, mockAction.sendCalled)
	assert.Equal(t, "C12345", mockAction.dest)
}

// --- Test for JSON marshal error in Send (line 259) ---

func TestSendWithUnmarshalableData(t *testing.T) {
	jr := NewJSON()
	jr.Code = 200
	jr.Message = "OK"
	jr.Data = make(chan int)

	rec := httptest.NewRecorder()
	jr.Send(rec)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --- Test for io.Copy error in sendExport (line 277) ---

func TestSendExportWriteError(t *testing.T) {
	buf := bytes.NewBufferString("file content")
	jr := NewJSON().Export("test.csv", buf)

	fw := &failWriter{}
	jr.sendExport(fw)
	assert.Equal(t, "attachment; filename=test.csv", fw.Header().Get("Content-Disposition"))
}

// --- Test for write error after JSON marshal failure in Send (line 262) ---

func TestSendWithMarshalAndWriteError(t *testing.T) {
	jr := NewJSON()
	jr.Code = 200
	jr.Message = "OK"
	jr.Data = make(chan int)

	fw := &failWriter{}
	jr.Send(fw)
	// The response should be a 500 because json.Marshal failed
	// The inner writeErr is also triggered because failWriter fails on Write
}
