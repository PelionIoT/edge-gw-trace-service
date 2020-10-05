package api

import (
	"encoding/json"
	"fmt"
	"github.com/armPelionEdge/wigwag-go-logger/logging"
	"net/http"
	"net/url"
)

type MbedAPIClientFactoryImpl struct {
	URL *url.URL
}

func (mbedAPIClientFactory *MbedAPIClientFactoryImpl) CreateMbedAPIClient(header http.Header) MbedAPI {
	return &MbedAPIImpl{
		header: header,
		url: mbedAPIClientFactory.URL,
	}
}

type MbedAPIImpl struct {
	header http.Header
	url *url.URL
}

func (mbedAPI *MbedAPIImpl) Device(deviceID string) MbedDevice {
	return &MbedDeviceImpl{
		deviceID: deviceID,
		url: mbedAPI.url,
		header: mbedAPI.header,
	}
}

type MbedDeviceImpl struct {
	deviceID string
	url *url.URL
	header http.Header
}

func (mbedDevice *MbedDeviceImpl) Get() (MbedDeviceStatus, error) {
	relativePath, err := url.Parse(fmt.Sprintf("./v3/devices/%s", mbedDevice.deviceID))

	if err != nil {
		logging.Log.Criticalf("Error creating relaive path: %v", err)

		return MbedDeviceStatus{}, err
	}

	request, err := http.NewRequest("GET", mbedDevice.url.ResolveReference(relativePath).String(), nil)

	if err != nil {
		logging.Log.Criticalf("Error creating GET request: %v", err)

		return MbedDeviceStatus{}, err
	}

	request.Header = mbedDevice.header
	response, err := http.DefaultClient.Do(request)

	if err != nil {
		logging.Log.Errorf("Error requesting device attributes for device %s: %v", mbedDevice.deviceID, err)

		return MbedDeviceStatus{}, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return MbedDeviceStatus{}, APIError{ Status: response.StatusCode }
	}

	var mbedDeviceStatus MbedDeviceStatus
	var decoder *json.Decoder = json.NewDecoder(response.Body)

	if err := decoder.Decode(&mbedDeviceStatus); err != nil {
		logging.Log.Errorf("Error requesting device attributes: Unable to decode response: %v", err)

		return MbedDeviceStatus{}, err
	}

	return mbedDeviceStatus, nil
}
