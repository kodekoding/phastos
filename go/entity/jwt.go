package entity

import "github.com/golang-jwt/jwt/v4"

type JWTClaimData struct {
	Data  interface{}
	Token string
	jwt.RegisteredClaims
}
