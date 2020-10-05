package middleware

import (
	"context"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/armPelionEdge/wigwag-go-logger/logging"
	"github.com/armPelionEdge/edge-gw-services-go/adapters"
	"github.com/armPelionEdge/edge-gw-services-go/token"
)

type ArmAccessTokenGetter interface {
	GetAccessToken(r *http.Request) string
}

type ArmAccessTokenDecoder interface {
	DecodeAccessToken(t string) (token.ArmAccessToken, error)
}

func ArmAccessTokenMiddleware(AccessTokenGetter ArmAccessTokenGetter, AccessTokenDecoder ArmAccessTokenDecoder) mux.MiddlewareFunc {
	return mux.MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			encodedToken := AccessTokenGetter.GetAccessToken(r)

			if encodedToken == "" {
				logging.Log.Errorf("%s %s: No ARM access token provided. Unauthorized", r.Method, r.RequestURI)
				
				adapters.MbedErrorResponse(w, r, adapters.MbedError{ Code: http.StatusUnauthorized, Type: "invalid_auth", Message: "No access token provided" })

				return
			}

			token, err := AccessTokenDecoder.DecodeAccessToken(encodedToken)

			if err != nil {
				logging.Log.Errorf("%s %s: Invalid ARM access token provided %s: %v. Bad Request.", r.Method, r.RequestURI, encodedToken, err)

				adapters.MbedErrorResponse(w, r, adapters.MbedError{ Code: http.StatusUnauthorized, Type: "invalid_token", Message: "Unable to decode token" })

				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ArmAccessTokenContextKey, token)))
		})
	})
}