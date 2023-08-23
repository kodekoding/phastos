package helper

import (
	"github.com/golang-jwt/jwt/v4"
	"github.com/kodekoding/phastos/v2/go/entity"
	"github.com/pkg/errors"
	"os"
	"time"
)

func GenerateJWTToken(data interface{}) (string, error) {
	nowTime := time.Now()
	expireTime := nowTime.Add(24 * time.Hour)

	defaultIssuer := "phastos"
	jwtIssuer := os.Getenv("JWT_ISSUER")
	if jwtIssuer == "" {
		jwtIssuer = defaultIssuer
	}
	claimData := new(entity.JWTClaimData)
	claimData.Data = data
	claimData.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expireTime),
		Issuer:    jwtIssuer,
	}

	tokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, claimData)
	token, err := tokenClaims.SignedString([]byte(os.Getenv("JWT_SIGNING_KEY")))
	if err != nil {
		return "", errors.Wrap(err, "lib.helper.jwt.GenerateJWTToken.SignedString")
	}
	return token, nil
}
