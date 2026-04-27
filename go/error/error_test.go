package error

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	t.Run("should create request error from error", func(t *testing.T) {
		err := errors.New("something failed")
		reqErr := New(err)
		assert.NotNil(t, reqErr)
		assert.Equal(t, "something failed", reqErr.Error())
		assert.NotNil(t, reqErr.GetData())
		assert.Empty(t, reqErr.GetData())
	})
}

func TestRequestError_SetCode(t *testing.T) {
	reqErr := New(errors.New("test"))
	result := reqErr.SetCode(400)
	assert.Equal(t, 400, result.GetCode())
	// Should return self for chaining
	assert.Equal(t, reqErr, result)
}

func TestRequestError_SetMessage(t *testing.T) {
	reqErr := New(errors.New("test"))
	result := reqErr.SetMessage("custom message")
	assert.Equal(t, "custom message", result.GetMessage())
	assert.Equal(t, reqErr, result)
}

func TestRequestError_AppendData(t *testing.T) {
	reqErr := New(errors.New("test"))

	reqErr.AppendData("field1", "value1")
	reqErr.AppendData("field2", 42)

	data := reqErr.GetData()
	assert.Equal(t, "value1", data["field1"])
	assert.Equal(t, 42, data["field2"])
	assert.Len(t, data, 2)
}

func TestRequestError_Chaining(t *testing.T) {
	reqErr := New(errors.New("validation failed")).
		SetCode(422).
		SetMessage("invalid input").
		AppendData("field", "email").
		AppendData("reason", "format invalid")

	assert.Equal(t, 422, reqErr.GetCode())
	assert.Equal(t, "invalid input", reqErr.GetMessage())
	assert.Equal(t, "validation failed", reqErr.Error())
	assert.Equal(t, "email", reqErr.GetData()["field"])
	assert.Equal(t, "format invalid", reqErr.GetData()["reason"])
}

func TestRequestError_Error(t *testing.T) {
	t.Run("should return underlying error message", func(t *testing.T) {
		reqErr := New(errors.New("original error"))
		assert.Equal(t, "original error", reqErr.Error())
	})
}

func TestRequestError_GetCode_Default(t *testing.T) {
	reqErr := New(errors.New("test"))
	assert.Equal(t, 0, reqErr.GetCode())
}

func TestRequestError_GetMessage_Default(t *testing.T) {
	reqErr := New(errors.New("test"))
	assert.Equal(t, "", reqErr.GetMessage())
}
