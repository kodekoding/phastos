package common

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestErrorConstants(t *testing.T) {
	assert.EqualError(t, ErrStructNotCompatible, "struct is not compatible")
	assert.EqualError(t, ErrIdParamIsRequired, "parameter ID is required")
	assert.EqualError(t, ErrIdMustNumeric, "ID must be defined as number")
	assert.EqualError(t, ErrPatch, "error patch")
}

func TestErrorConstantsAreNotNil(t *testing.T) {
	errs := []error{ErrStructNotCompatible, ErrIdParamIsRequired, ErrIdMustNumeric, ErrPatch}
	for _, e := range errs {
		assert.NotNil(t, e)
	}
}

func TestErrorConstantsType(t *testing.T) {
	// verify they are errors package errors (support errors.Is/Cause)
	var _ error = ErrStructNotCompatible
	var _ error = ErrIdParamIsRequired
	var _ error = ErrIdMustNumeric
	var _ error = ErrPatch
}

func TestErrorWrapping(t *testing.T) {
	wrapped := errors.Wrap(ErrStructNotCompatible, "wrapper")
	assert.True(t, errors.Is(wrapped, ErrStructNotCompatible))
}

func TestStringConstants(t *testing.T) {
	assert.Equal(t, "requestId", RequestIdContextKey)
	assert.Equal(t, "invalid token", ErrInvalidTokenMessage)
	assert.Equal(t, "INVALID_TOKEN", ErrInvalidTokenCode)
	assert.Equal(t, "secret", HeaderSecret)
	assert.Equal(t, "SERVICE_SECRET", EnvServiceSecret)
	assert.Equal(t, "JWT_SIGNING_KEY", EnvJWTSigningKey)
}
