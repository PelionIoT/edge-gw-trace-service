package middleware

import (
	"net/http"
	"github.com/gorilla/mux"
	"github.com/armPelionEdge/wigwag-go-logger/logging"
	"github.com/armPelionEdge/edge-gw-services-go/adapters"
	"github.com/armPelionEdge/edge-gw-services-go/middleware/mbed"
	"github.com/armPelionEdge/edge-gw-services-go/token"
)

func DeviceIDTranslator(deviceIdTranslatorFactory mbed.DeviceIdTranslatorFactory) mux.MiddlewareFunc {
	return mux.MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			armAccessToken, _ := r.Context().Value(ArmAccessTokenContextKey).(token.ArmAccessToken)

			vars := mux.Vars(r)

			device, ok := vars["device"]
			
			if !ok {
				logging.Log.Debugf("[ RequestID:%s,AccountID:%s ] No device path parameter found. Nothing to translate", armAccessToken.RequestID, armAccessToken.AccountID)

				next.ServeHTTP(w, r)

				return
			}

			logging.Log.Debugf("[ RequestID:%s,AccountID:%s ] Translating Mbed device ID %s to Wigwag relay ID", armAccessToken.RequestID, armAccessToken.AccountID, device)
			
			relayId, err := deviceIdTranslatorFactory.CreateDeviceIdTranslator(r.Header).MbedDeviceIdToWigwagRelayId(device)

			if err != nil {
				logging.Log.Errorf("[ RequestID:%s,AccountID:%s ] Unable to translate Mbed device ID %s to Wigwag relay ID: %v", armAccessToken.RequestID, armAccessToken.AccountID, device, err)

				if err == mbed.EMappingNotFound {
					adapters.MbedErrorResponse(w, r, adapters.MbedError{ Code: http.StatusNotFound, Type: "not_found", Message: "Unknown device ID. It could not be found in the mapping database." })

					return
				}

				adapters.MbedErrorResponse(w, r, adapters.MbedError{ Code: http.StatusInternalServerError, Type: "internal_server_error", Message: "Unable to access device mapping database." })

				return
			}

			logging.Log.Debugf("[ RequestID:%s,AccountID:%s ] Translated Mbed device ID %s to Wigwag relay ID %s", armAccessToken.RequestID, armAccessToken.AccountID, device, relayId)

			vars["original_device"] = device
			vars["device"] = relayId

			next.ServeHTTP(w, r)
		})
	})
}