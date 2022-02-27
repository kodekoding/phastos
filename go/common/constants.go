package common

import "github.com/pkg/errors"

var (
	ErrStructNotCompatible = errors.New("struct is not compatible")
	ErrIdParamIsRequired   = errors.New("parameter ID is required")
	ErrIdMustNumeric       = errors.New("ID must be defined as number")
)
