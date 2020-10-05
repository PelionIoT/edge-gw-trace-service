package tokens

import (
	"crypto/rsa"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type JWTTokenFactory struct {
	Issuer     string
	TokenExp   time.Duration
	SigningKey *rsa.PrivateKey
}

func (tokenFactory *JWTTokenFactory) initClaims(claims map[string]interface{}) map[string]interface{} {
	if claims == nil {
		claims = map[string]interface{}{}
	}

	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Duration(time.Now().Add(tokenFactory.TokenExp).UnixNano()) / time.Second
	}

	claims["iss"] = tokenFactory.Issuer

	return claims
}

func (tokenFactory *JWTTokenFactory) CreateAccountToken(accountID string, claims map[string]interface{}) (string, error) {
	claims = tokenFactory.initClaims(claims)
	claims["sub"] = "account"
	claims["account_id"] = accountID

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims))

	return token.SignedString(tokenFactory.SigningKey)
}
