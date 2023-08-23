package context

import (
	"context"
	"net/http"

	"github.com/kodekoding/phastos/v2/go/entity"
)

type (
	jwtContext struct{}
)

func SetJWT(req *http.Request, jwtData *entity.JWTClaimData) {
	ctx := context.WithValue(req.Context(), jwtContext{}, jwtData)
	*req = *req.WithContext(ctx)
}

func GetJWT(ctx context.Context) *entity.JWTClaimData {
	jwtData, ok := ctx.Value(jwtContext{}).(*entity.JWTClaimData)
	if !ok {
		return nil
	}
	return jwtData
}
