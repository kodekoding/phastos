package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewErr(t *testing.T) {
	t.Run("should create default error", func(t *testing.T) {
		err := NewErr()
		assert.Equal(t, http.StatusInternalServerError, err.Status)
		assert.Equal(t, "SERVER_ERROR", err.Code)
		assert.Equal(t, "an error occured", err.Message)
	})

	t.Run("should apply options", func(t *testing.T) {
		err := NewErr(
			WithErrorCode("CUSTOM_CODE"),
			WithErrorStatus(http.StatusBadRequest),
			WithErrorMessage("custom message"),
			WithErrorData(map[string]string{"key": "value"}),
			WithTraceId("trace-123"),
			WithErrorCallerPath("api.handler.Create"),
		)
		assert.Equal(t, http.StatusBadRequest, err.Status)
		assert.Equal(t, "CUSTOM_CODE", err.Code)
		assert.Equal(t, "custom message", err.Message)
		assert.Equal(t, map[string]string{"key": "value"}, err.Data)
		assert.Equal(t, "trace-123", err.TraceId)
		assert.Equal(t, "api.handler.Create", err.CallerPath)
	})
}

func TestHttpError_Error(t *testing.T) {
	err := NewErr(WithErrorMessage("something went wrong"))
	assert.Equal(t, "something went wrong", err.Error())
}

func TestInternalServerError(t *testing.T) {
	err := InternalServerError("db connection failed", "DB_ERROR")
	assert.Equal(t, 500, err.Status)
	assert.Equal(t, "DB_ERROR", err.Code)
	assert.Equal(t, "db connection failed", err.Message)
}

func TestUnprocessableEntity(t *testing.T) {
	err := UnprocessableEntity("invalid data", "VALIDATION_ERROR")
	assert.Equal(t, 422, err.Status)
	assert.Equal(t, "VALIDATION_ERROR", err.Code)
	assert.Equal(t, "invalid data", err.Message)
}

func TestNotFound(t *testing.T) {
	err := NotFound("resource not found", "NOT_FOUND")
	assert.Equal(t, 404, err.Status)
	assert.Equal(t, "NOT_FOUND", err.Code)
	assert.Equal(t, "resource not found", err.Message)
}

func TestMethodNotAllowed(t *testing.T) {
	err := MethodNotAllowed("method not allowed", "METHOD_NOT_ALLOWED")
	assert.Equal(t, 405, err.Status)
	assert.Equal(t, "METHOD_NOT_ALLOWED", err.Code)
	assert.Equal(t, "method not allowed", err.Message)
}

func TestUnauthorized(t *testing.T) {
	err := Unauthorized("invalid token", "INVALID_TOKEN")
	assert.Equal(t, 401, err.Status)
	assert.Equal(t, "INVALID_TOKEN", err.Code)
	assert.Equal(t, "invalid token", err.Message)
}

func TestForbidden(t *testing.T) {
	err := Forbidden("access denied", "FORBIDDEN")
	assert.Equal(t, 403, err.Status)
	assert.Equal(t, "FORBIDDEN", err.Code)
	assert.Equal(t, "access denied", err.Message)
}

func TestBadRequest(t *testing.T) {
	err := BadRequest("missing field", "MISSING_FIELD")
	assert.Equal(t, 400, err.Status)
	assert.Equal(t, "MISSING_FIELD", err.Code)
	assert.Equal(t, "missing field", err.Message)
}

func TestTooManyRequest(t *testing.T) {
	err := TooManyRequest("rate limit exceeded", "RATE_LIMIT")
	assert.Equal(t, 429, err.Status)
	assert.Equal(t, "RATE_LIMIT", err.Code)
	assert.Equal(t, "rate limit exceeded", err.Message)
}
