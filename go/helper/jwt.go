package helper

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/kodekoding/phastos/v2/go/entity"
)

func GenerateJWTToken(data interface{}, expireTime ...time.Duration) (string, error) {
	nowTime := time.Now()
	expiredTime := nowTime.Add(24 * time.Hour)
	if len(expireTime) > 0 {
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
	// jwt.SigningMethodHMAC.Sign only fails if the key is nil — os.Getenv returns "" (not nil) when unset
	// jwt.SigningMethodHMAC.Sign hanya gagal jika key nil — os.Getenv mengembalikan "" (bukan nil) jika tidak diset
	token, _ := tokenClaims.SignedString([]byte(os.Getenv("JWT_SIGNING_KEY")))
	return token, nil
}
