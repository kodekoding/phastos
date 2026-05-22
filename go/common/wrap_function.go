package common

import (
	"context"

	"github.com/kodekoding/phastos/v2/go/database"
)

type (
	GetReturn func(ctx context.Context) (*database.SelectResponse, error)
	CUDReturn func(ctx context.Context) (*database.CUDResponse, error)
)
