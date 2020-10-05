package storage

import (
	"errors"
	"github.com/armPelionEdge/wigwag-go-logger/logging"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"sync"
	"time"
)

const retryBackoffSeconds int = 5
var ENotInitialized error = errors.New("MongoDB session not initialized")
var ENotFound error = errors.New("No mapping found")

type MongoDeviceMappingStore struct {
	MongoURI *mgo.DialInfo
	CollectionName string
	parentSession *mgo.Session
	setupLock sync.RWMutex
}

func (deviceMappingStore *MongoDeviceMappingStore) Setup() error {
	for deviceMappingStore.parentSession == nil {
		deviceMappingStore.setupLock.Lock()

		logging.Log.Debugf("MongoDeviceMappingStore attempting to establish MongoDB session: %v", deviceMappingStore.MongoURI.Addrs)

		deviceMappingStore.MongoURI.Timeout = time.Second * 10

		session, err := mgo.DialWithInfo(deviceMappingStore.MongoURI)

		if err != nil {
			deviceMappingStore.setupLock.Unlock()

			logging.Log.Criticalf("MongoDeviceMappingStore unable to establish MongoDB session. Will retry in %d seconds: %v", retryBackoffSeconds, err)

			<-time.After(time.Second * time.Duration(retryBackoffSeconds))

			continue
		}

		session.SetMode(mgo.Monotonic, true)
		collection := session.DB("").C(deviceMappingStore.CollectionName)

		logging.Log.Debugf("MongoDeviceMappingStore MongoDB session established. Ensuring indexes on collection %s", deviceMappingStore.CollectionName)

		if err := deviceMappingStore.ensureIndexes(collection); err != nil {
			deviceMappingStore.setupLock.Unlock()
			session.Close()

			logging.Log.Criticalf("MongoDeviceMappingStore unable to ensure indexes. Will retry in %d seconds: %v", retryBackoffSeconds, err)

			<-time.After(time.Second * time.Duration(retryBackoffSeconds))

			continue
		}

		logging.Log.Debugf("MongoDeviceMappingStore Ensured indexes on collection %s", deviceMappingStore.CollectionName)

		deviceMappingStore.parentSession = session

		deviceMappingStore.setupLock.Unlock()
	}

	return nil
}

func (deviceMappingStore *MongoDeviceMappingStore) ensureIndexes(collection *mgo.Collection) error {
	var indexes = []mgo.Index{
		mgo.Index{ 
			Key: []string{ "relay_id" }, 
			Unique: true,
			DropDups: true,
			Background: true,
			Sparse: true,
		},
		mgo.Index{ 
			Key: []string{ "device_id" }, 
			Unique: true,
			DropDups: true,
			Background: true,
			Sparse: true,
		},
	}

	for _, index := range indexes {
		logging.Log.Debugf("MongoDeviceMappingStore Ensuring index %s", index.Key[0])

		if err := collection.EnsureIndex(index); err != nil {
			return err
		}
	}

	return nil
}

func (deviceMappingStore *MongoDeviceMappingStore) session() (*mgo.Session, *mgo.Collection) {
	session := deviceMappingStore.parentSession.Copy()
	collection := session.DB("").C(deviceMappingStore.CollectionName)

	return session, collection
}

func (deviceMappingStore *MongoDeviceMappingStore) PutMapping(deviceMapping DeviceMappingMetadata) error {
	deviceMappingStore.setupLock.RLock()
	defer deviceMappingStore.setupLock.RUnlock()

	if deviceMappingStore.parentSession == nil {
		return ENotInitialized
	}

	session, collection := deviceMappingStore.session()
	defer session.Close()

	_, err := collection.Upsert(bson.M{ "relay_id": deviceMapping.RelayID }, bson.M{ "$set": deviceMapping })

	return err
}

func (deviceMappingStore *MongoDeviceMappingStore) GetMappingByRelayId(relayId string) (DeviceMappingMetadata, error) {
	return deviceMappingStore.getOne(bson.M{ "relay_id": relayId })
}

func (deviceMappingStore *MongoDeviceMappingStore) GetMappingByDeviceId(deviceId string) (DeviceMappingMetadata, error) {
	return deviceMappingStore.getOne(bson.M{ "device_id": deviceId })
}

func (deviceMappingStore *MongoDeviceMappingStore) getOne(query interface{}) (DeviceMappingMetadata, error) {
	deviceMappingStore.setupLock.RLock()
	defer deviceMappingStore.setupLock.RUnlock()

	if deviceMappingStore.parentSession == nil {
		return DeviceMappingMetadata{}, ENotInitialized
	}

	var deviceMappingMetadata DeviceMappingMetadata

	session, collection := deviceMappingStore.session()
	defer session.Close()

	err := collection.Find(query).One(&deviceMappingMetadata)

	if err == mgo.ErrNotFound {
		return deviceMappingMetadata, ENotFound
	}

	return deviceMappingMetadata, err
}