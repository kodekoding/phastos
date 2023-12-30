package context

import (
	"context"
	"net/http"

	"github.com/kodekoding/phastos/v2/go/entity"
)

type (
	JwtContext struct{}
)

func SetJWT(req *http.Request, jwtData *entity.JWTClaimData) {
	ctx := context.WithValue(req.Context(), JwtContext{}, jwtData)
	*req = *req.WithContext(ctx)
}

func GetJWT(ctx context.Context) *entity.JWTClaimData {
	jwtData, ok := ctx.Value(JwtContext{}).(*entity.JWTClaimData)
	if !ok {
		return nil
	}
	return jwtData
}
