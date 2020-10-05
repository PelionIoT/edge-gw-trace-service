package storage

type DeviceMappingMetadata struct {
	RelayID string `bson:"relay_id" json:"relay_id"`
	DeviceID string `bson:"device_id" json:"device_id"`
}

type DeviceMappingStore interface {
	Setup() error
	PutMapping(deviceMapping DeviceMappingMetadata) error
	GetMappingByRelayId(relayId string) (DeviceMappingMetadata, error)
	GetMappingByDeviceId(deviceId string) (DeviceMappingMetadata, error)
}