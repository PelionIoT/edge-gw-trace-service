package wigwag

import (
	"encoding/json"
	"github.com/armPelionEdge/wigwag-go-logger/logging"
	"github.com/armPelionEdge/edge-gw-services-go/token"
	"net/http"
)

const XWigwagIdentityHeaderKey = "X-Wigwag-Identity"
const XWigwagRelayIDHeaderKey = "X-Wigwag-RelayID"
const XWigwagRequestIDHeaderKey = "X-Request-ID"
const XWigwagAccountIDHeaderKey = "X-Account-ID"
const AuthorizationHeaderKey = "Authorization"
const XWigwagAuthenticatedHeaderKey = "X-Wigwag-Authenticated"

type WigwagAccessHeaderWriterImpl struct {
}

func (accessHeaderWriter *WigwagAccessHeaderWriterImpl) WriteAccessHeaders(accountID string, r *http.Request) {
	var identity token.WigwagIdentityHeader

	identity.AccountID = accountID
	identity.ClientType = "user_internal"

	encodedHeader, err := json.Marshal(identity)

	if err != nil {
		logging.Log.Panicf("Unable to encode x-wigwag-identity header: %v", err)
	}

	r.Header.Del(AuthorizationHeaderKey)
	r.Header.Set(XWigwagIdentityHeaderKey, string(encodedHeader))
	r.Header.Set(XWigwagAuthenticatedHeaderKey, "true")
}
