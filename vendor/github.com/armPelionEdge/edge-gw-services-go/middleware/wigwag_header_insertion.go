package middleware

import (
	"net/http"
	"github.com/gorilla/mux"
	"github.com/armPelionEdge/wigwag-go-logger/logging"
	"github.com/armPelionEdge/edge-gw-services-go/adapters"
	"github.com/armPelionEdge/edge-gw-services-go/token"
)

type WigwagAccessHeaderWriter interface {
	WriteAccessHeaders(accountID string, r *http.Request)
}

func WigwagHeaderInsertionMiddleware(WigwagAccessHeaderWriter WigwagAccessHeaderWriter) mux.MiddlewareFunc {
	return mux.MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			armAccessToken, ok := r.Context().Value(ArmAccessTokenContextKey).(token.ArmAccessToken)

			if !ok {
				logging.Log.Errorf("[ RequestID:%s,AccountID:%s ] Wigwag header insertion middleware expects an access token in the request context but none was provided. Bad request", armAccessToken.RequestID, armAccessToken.AccountID)

				adapters.MbedErrorResponse(w, r, adapters.MbedError{ Code: http.StatusBadRequest, Type: "bad_request", Message: "Unable to retrieve access token from request context." })

				return
			}
			
			WigwagAccessHeaderWriter.WriteAccessHeaders(armAccessToken.AccountID, r)

			next.ServeHTTP(w, r)
		})
	})
}