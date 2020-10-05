package mbed

import (
	"net/http"
)

type NopDeviceIdTranslatorFactory struct {
}

func (deviceIdTranslatorFactory *NopDeviceIdTranslatorFactory) CreateDeviceIdTranslator(header http.Header) DeviceIdTranslator {
	return &NopDeviceIdTranslatorFactory{}
}

type NopDeviceIdTranslator struct {
}

func (deviceIdTranslator *NopDeviceIdTranslatorFactory) MbedDeviceIdToWigwagRelayId(deviceId string) (string, error) {
	return deviceId, nil
}