package monitoring

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithAppName(t *testing.T) {
	opt := WithAppName("test-app")
	nr := &newRelic{}
	opt(nr)
	assert.Equal(t, "test-app", nr.appName)
}

func TestWithLicenseKey(t *testing.T) {
	opt := WithLicenseKey("test-license")
	nr := &newRelic{}
	opt(nr)
	assert.Equal(t, "test-license", nr.licenseKey)
}

func TestBeginTrxFromContextEmpty(t *testing.T) {
	ctx := context.Background()
	txn := BeginTrxFromContext(ctx)
	assert.Nil(t, txn)
}

func TestNewContext(t *testing.T) {
	ctx := context.Background()
	// Passing a nil transaction is valid - it just stores nil in context
	newCtx := NewContext(ctx, nil)
	assert.NotNil(t, newCtx)
}

func TestNewRelicStruct(t *testing.T) {
	nr := &newRelic{
		appName:    "test",
		licenseKey: "key123",
	}
	assert.Equal(t, "test", nr.appName)
	assert.Equal(t, "key123", nr.licenseKey)
}

func TestNewRelicGetAppNil(t *testing.T) {
	nr := &newRelic{}
	assert.Nil(t, nr.GetApp())
}

func TestNewRelicsInterface(t *testing.T) {
	// Verify the NewRelics interface is defined (empty but still valid)
	var _ NewRelics = &newRelic{}
}
