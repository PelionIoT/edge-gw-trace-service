package access_tokens

import (
	"crypto/rsa"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/armPelionEdge/edge-gw-services-go/token"
)

type ArmAccessTokenDecoderImpl struct {
	PublicKey *rsa.PublicKey
}

func (accessTokenDecoder *ArmAccessTokenDecoderImpl) DecodeAccessToken(t string) (token.ArmAccessToken, error) {
	tkn, err := jwt.Parse(t, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return accessTokenDecoder.PublicKey, nil
	})

	if err != nil {
		return token.ArmAccessToken{}, err
	}

	if !tkn.Valid {
		return token.ArmAccessToken{}, fmt.Errorf("Token could not be validated")
	}

	var accessToken token.ArmAccessToken

	if claims, ok := tkn.Claims.(jwt.MapClaims); ok {
		if accountID, ok := claims["account_id"].(string); ok {
			accessToken.AccountID = accountID
		}

		if requestID, ok := claims["request_id"].(string); ok {
			accessToken.RequestID = requestID
		}
	}

	if accessToken.AccountID == "" {
		return token.ArmAccessToken{}, fmt.Errorf("Access token contained no account ID")
	}

	return accessToken, nil
}