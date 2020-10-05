package middleware

import (
	"net/http"
	"github.com/gorilla/mux"
	"github.com/armPelionEdge/wigwag-go-logger/logging"
	"github.com/armPelionEdge/edge-gw-services-go/token"
)

const ArmAccessTokenContextKey string = "armAccessToken"

func RequestLoggerMiddleware() mux.MiddlewareFunc {
	return mux.MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			armAccessToken, ok := r.Context().Value(ArmAccessTokenContextKey).(token.ArmAccessToken)

			if ok {
				logging.Log.Debugf("[ RequestID:%s,AccountID:%s ] %s %s", armAccessToken.RequestID, armAccessToken.AccountID, r.Method, r.URL.RequestURI())
			}

			next.ServeHTTP(w, r)
		})
	})
}