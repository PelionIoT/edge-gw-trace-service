package mbed

import (
	"errors"
	"net/http"
)

var EMappingNotFound error = errors.New("The device mapping was not found")
var EInternalError error = errors.New("Internal error")

type DeviceIdTranslatorFactory interface {
	CreateDeviceIdTranslator(header http.Header) DeviceIdTranslator
}

type DeviceIdTranslator interface {
	MbedDeviceIdToWigwagRelayId(deviceId string) (string, error)
}
