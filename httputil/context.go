package httputil

import (
	"context"
	"net/http"
)

const ContextKeyRequestID = "request_id"
const ContextKeyAccountID = "account_id"

func WithContextValue(r *http.Request, key string, value string) (*http.Request, string) {
	r = r.WithContext(context.WithValue(r.Context(), key, value))

	return r, value
}
