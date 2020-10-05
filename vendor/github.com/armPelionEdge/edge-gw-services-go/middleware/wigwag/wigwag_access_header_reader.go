package wigwag

import (
	"encoding/json"
	"net/http"

	"github.com/armPelionEdge/edge-gw-services-go/token"
	"github.com/armPelionEdge/wigwag-go-logger/logging"
)

type WigwagAccessHeaderReaderImpl struct {
}

func (accessHeaderReader *WigwagAccessHeaderReaderImpl) ReadAccessHeaders(r *http.Request) (token.WigwagIdentityHeader, error) {
	var identity token.WigwagIdentityHeader

	err := json.Unmarshal([]byte(r.Header.Get(XWigwagIdentityHeaderKey)), &identity)

	if err != nil {
		logging.Log.Criticalf("Unable to decode x-wigwag-identity header: %v", err)

		return token.WigwagIdentityHeader{}, err
	}

	return identity, nil
}
