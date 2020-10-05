package mbed

import (
	"github.com/armPelionEdge/wigwag-go-logger/logging"
	"github.com/armPelionEdge/edge-gw-services-go/api"
	"net/http"
)

type EndpointNameDeviceIdTranslatorFactory struct {
	MbedAPIClientFactory api.MbedAPIClientFactory
}

func (deviceIdTranslatorFactory *EndpointNameDeviceIdTranslatorFactory) CreateDeviceIdTranslator(header http.Header) DeviceIdTranslator {
	return &EndpointNameDeviceIdTranslator{
		MbedAPI: deviceIdTranslatorFactory.MbedAPIClientFactory.CreateMbedAPIClient(header),
	}
}

type EndpointNameDeviceIdTranslator struct {
	MbedAPI api.MbedAPI
}

func (deviceIdTranslator *EndpointNameDeviceIdTranslator) MbedDeviceIdToWigwagRelayId(deviceId string) (string, error) {
	deviceStatus, err := deviceIdTranslator.MbedAPI.Device(deviceId).Get()

	if err != nil {
		logging.Log.Debugf("Unable to translate Mbed device ID %s to Wigwag relay ID: %v", deviceId, err)

		apiError, ok := err.(api.APIError)

		if ok && apiError.Status == http.StatusNotFound {
			return "", EMappingNotFound
		}

		return "", EInternalError
	}

	return deviceStatus.EndpointName, nil
}