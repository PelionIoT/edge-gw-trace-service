package muuid

import (
	"encoding/hex"
	"errors"
	"net"
	"sync"
	"time"
)

var ENetworkInterfaceNotFound error = errors.New("Could not retrieve a MAC address from the specified interface")
type MUUID [16]byte
type Timestamp [6]byte
type MACAddress [6]byte

func (muuid MUUID) String() string {
	return hex.EncodeToString(muuid[:])
}

type MUUIDGenerator struct {
	lastTimestamp Timestamp
	lastTimestampMs time.Duration
	MAC MACAddress
	InstanceId uint16
	SequenceNumber uint32
	mu sync.Mutex
}

func (generator *MUUIDGenerator) UUID() MUUID {
	generator.mu.Lock()
	defer generator.mu.Unlock()

	var uuid MUUID
	var now Timestamp = generator.timestampMilliseconds()

	copy(uuid[0:6], now[0:6])
	copy(uuid[6:12], generator.MAC[0:6])

	uuid[12] = uint8(generator.InstanceId >> 4) & 0xFF
	uuid[13] = (uint8(generator.InstanceId << 4) & 0xF0) | (uint8(generator.SequenceNumber >> 16) & 0x0F)
	uuid[14] = uint8(generator.SequenceNumber >> 8) & 0xFF
	uuid[15] = uint8(generator.SequenceNumber) & 0xFF

	return uuid
}

func (generator *MUUIDGenerator) UUIDFromSeqNum(SequenceNumber uint32) MUUID {
	generator.mu.Lock()
	defer generator.mu.Unlock()

	var uuid MUUID
	var now Timestamp = generator.timestampMilliseconds()

	copy(uuid[0:6], now[0:6])
	copy(uuid[6:12], generator.MAC[0:6])

	uuid[12] = uint8(generator.InstanceId >> 4) & 0xFF
	uuid[13] = (uint8(generator.InstanceId << 4) & 0xF0) | (uint8(SequenceNumber >> 16) & 0x0F)
	uuid[14] = uint8(SequenceNumber >> 8) & 0xFF
	uuid[15] = uint8(SequenceNumber) & 0xFF

	return uuid
}

func (generator *MUUIDGenerator) timestampMilliseconds() Timestamp {
	var now Timestamp

	nowMs := time.Nanosecond * time.Duration(time.Now().UnixNano()) / time.Millisecond

	if nowMs == generator.lastTimestampMs {
		generator.SequenceNumber++

		return generator.lastTimestamp
	}

	now[0] = uint8(nowMs >> 40) & 0xFF
	now[1] = uint8(nowMs >> 32) & 0xFF
	now[2] = uint8(nowMs >> 24) & 0xFF
	now[3] = uint8(nowMs >> 16) & 0xFF
	now[4] = uint8(nowMs >> 8) & 0xFF
	now[5] = uint8(nowMs) & 0xFF

	generator.SequenceNumber = 0
	generator.lastTimestampMs = nowMs
	generator.lastTimestamp = now

	return now
}

type MUUIDGeneratorBuilder struct {
	// Network interface name used to decide which MAC address is used in the MUUID generator
	NetworkInterface string
	InstanceId uint16
}

func (builder *MUUIDGeneratorBuilder) Build() (MUUIDGenerator, error) {
	mac, err := builder.getMACAddress()

	if err != nil {
		return MUUIDGenerator{}, err
	}

	return MUUIDGenerator{
		MAC: mac,
		InstanceId: builder.InstanceId,
	}, nil
}

func (builder *MUUIDGeneratorBuilder) getMACAddress() (MACAddress, error) {
	var macAddress MACAddress

	interfaces, err := net.Interfaces()

	if err != nil {
		return macAddress, err
	}

	for _, networkInterface := range interfaces {
		if networkInterface.Name == builder.NetworkInterface {
			copy(macAddress[:], networkInterface.HardwareAddr)

			return macAddress, nil
		}
	}

	return macAddress, ENetworkInterfaceNotFound
}