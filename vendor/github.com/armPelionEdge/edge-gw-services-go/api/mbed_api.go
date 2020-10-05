package api

import (
	"net/http"
)

type MbedAPIClientFactory interface {
	CreateMbedAPIClient(header http.Header) MbedAPI
}

type MbedAPI interface {
	Device(deviceID string) MbedDevice
}

type MbedDevice interface {
	Get() (MbedDeviceStatus, error)
}

type MbedDeviceStatus struct {
	EndpointName string `json:"endpoint_name"`
}