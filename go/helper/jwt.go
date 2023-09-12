package helper

import (
	"github.com/golang-jwt/jwt/v4"
	"github.com/kodekoding/phastos/v2/go/common"
	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/pkg/errors"
	"os"
	"time"
)

func GenerateJWTToken(data interface{}, expireTime ...time.Duration) (string, error) {
	nowTime := time.Now()
	expiredTime := nowTime.Add(24 * time.Hour)
	if expireTime != nil && len(expireTime) > 0 {
		expiredTime = nowTime.Add(expireTime[0])
	}

	defaultIssuer := "phastos"
	jwtIssuer := os.Getenv("JWT_ISSUER")
	if jwtIssuer == "" {
		jwtIssuer = defaultIssuer
	}
	claimData := new(entity.JWTClaimData)
	claimData.Data = data
	claimData.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expiredTime),
		Issuer:    jwtIssuer,
	}

	tokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, claimData)
	token, err := tokenClaims.SignedString([]byte(os.Getenv(common.EnvJWTSigningKey)))
	if err != nil {
		return "", errors.Wrap(err, "lib.helper.jwt.GenerateJWTToken.SignedString")
	}
	return token, nil
}
