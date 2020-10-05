package mbed

import (
	"net/http"
	"github.com/armPelionEdge/edge-gw-services-go/storage"
)

type MappingAPIDeviceIdTranslatorFactory struct {
	DeviceMappingStore storage.DeviceMappingStore
}

func (deviceIdTranslatorFactory *MappingAPIDeviceIdTranslatorFactory) CreateDeviceIdTranslator(header http.Header) DeviceIdTranslator {
	return &MappingAPIDeviceIdTranslator{
		DeviceMappingStore: deviceIdTranslatorFactory.DeviceMappingStore,
	}
}

type MappingAPIDeviceIdTranslator struct {
	DeviceMappingStore storage.DeviceMappingStore
}

func (deviceIdTranslator *MappingAPIDeviceIdTranslator) MbedDeviceIdToWigwagRelayId(deviceId string) (string, error) {
	deviceMapping, err := deviceIdTranslator.DeviceMappingStore.GetMappingByDeviceId(deviceId)

	if err == storage.ENotFound {
		return "", EMappingNotFound
	} else if err != nil {
		return "", err
	}

	return deviceMapping.RelayID, nil
}