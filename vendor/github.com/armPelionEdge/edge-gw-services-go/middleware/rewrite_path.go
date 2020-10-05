package middleware

import (
	"github.com/gorilla/mux"
	"github.com/armPelionEdge/wigwag-go-logger/logging"
	"github.com/armPelionEdge/edge-gw-services-go/token"
	"net/http"
	"path"
	"strings"
)

func RewritePathMiddleware(prefix string) mux.MiddlewareFunc {
	return mux.MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			armAccessToken, _ := r.Context().Value(ArmAccessTokenContextKey).(token.ArmAccessToken)

			if strings.HasPrefix(r.URL.Path, prefix) {
				logging.Log.Debugf("[ RequestID:%s,AccountID:%s ] Rewrite path for request from %s to %s", armAccessToken.RequestID, armAccessToken.AccountID, r.URL.Path, path.Join("/", r.URL.Path[len(prefix):]))

				r.URL.Path = path.Join("/", r.URL.Path[len(prefix):])
			}

			next.ServeHTTP(w, r)
		})
	})
}