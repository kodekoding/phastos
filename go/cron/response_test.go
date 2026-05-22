package cron

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewResponse(t *testing.T) {
	resp := NewResponse()
	assert.NotNil(t, resp)
	assert.Equal(t, "", resp.processName)
	assert.Nil(t, resp.err)
}

func TestResponseSetProcessName(t *testing.T) {
	resp := NewResponse()
	result := resp.SetProcessName("test-process")
	assert.Equal(t, "test-process", resp.processName)
	assert.Equal(t, resp, result) // returns self for chaining
}

func TestResponseSetError(t *testing.T) {
	resp := NewResponse()
	err := assert.AnError
	result := resp.SetError(err)
	assert.Equal(t, err, resp.err)
	assert.Equal(t, resp, result) // returns self for chaining
}

func TestResponseChaining(t *testing.T) {
	err := assert.AnError
	resp := NewResponse().SetProcessName("import-data").SetError(err)
	assert.Equal(t, "import-data", resp.processName)
	assert.Equal(t, err, resp.err)
}

func TestWithTimeZone(t *testing.T) {
	opt := WithTimeZone("Asia/Jakarta")
	assert.NotNil(t, opt)
	
	// Apply to option struct
	o := option{}
	opt(&o)
	assert.Equal(t, "Asia/Jakarta", o.timezone)
}

func TestWithTimeZoneEmpty(t *testing.T) {
	opt := WithTimeZone("")
	o := option{}
	opt(&o)
	assert.Equal(t, "", o.timezone)
}
