package common

import "github.com/pkg/errors"

var (
	ErrStructNotCompatible = errors.New("struct is not compatible")
	ErrIdParamIsRequired   = errors.New("parameter ID is required")
	ErrIdMustNumeric       = errors.New("ID must be defined as number")
	ErrPatch               = errors.New("error patch")
)

const (
	RequestIdContextKey    = "requestId"
	ErrInvalidTokenMessage = "invalid token"
	ErrInvalidTokenCode    = "INVALID_TOKEN"

	HeaderSecret = "secret"

	EnvServiceSecret = "SERVICE_SECRET"
	EnvJWTSigningKey = "JWT_SIGNING_KEY"
)
