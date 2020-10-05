package storage

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/armPelionEdge/edge-gw-services-go/adapters"
	"github.com/armPelionEdge/wigwag-go-logger/logging"
)

var (
	ErrGatewayNotFound        = errors.New("Gateway Not found")
	ErrGatewayUnexpectedFound = errors.New("Found unexpected number of gateway")
	ErrGatewayQueryFailed     = errors.New("Gateway registry query failed")
	ErrRegistryGatewayFailure = errors.New("Failed to registry the gateway")
)

type GatewaysRegistry interface {
	GetGateway(deviceID string) (GatewayImport, error)
	GatewaysExists(deviceID string) (bool, error)
	RegistryGateway(deviceID string, accountID string) error
}

type GatewayRegistryService struct {
	ServiceURI string `json:"service_uri"`
}

type GatewayImport struct {
	IP                string             `json:"ip_address"`
	AccountID         string             `json:"account_id"`
	SiteID            string             `json:"site_id"`
	PairingCode       string             `json:"pairing_code"`
	DeviceJsConnected bool               `json:"devicejs_connected"`
	DeviceDbConnected bool               `json:"devicedb_connected"`
	RelayID           string             `json:"relay_id"`
	HardwareVersion   string             `json:"hardware_version"`
	RadioConfig       string             `json:"radio_config"`
	EthernetMac       string             `json:"ethernet_mac"`
	SixBMac           string             `json:"sixb_mac"`
	ClientCert        string             `json:"client_cert"`
	ServerCert        string             `json:"server_cert"`
	Coordinates       []float64          `json:"coordinates"`
	Location          map[string]float64 `json:"location"`
	Latitude          float64            `json:"lat"`
	Longitude         float64            `json:"lon"`
}

type GetGatewayResponse struct {
	Object  string          `json:"object"`
	After   string          `json:"after"`
	Limit   int             `json:"limit"`
	Order   string          `json:"order"`
	HasMore bool            `json:"has_more"`
	Data    []GatewayImport `json:"data"`
}

type RegistryGatewayResponse struct {
	Code      int    `json:"code"`
	RequestID string `json:"request_id"`
	Object    string `json:"object"`
	Message   string `json:"message"`
	Type      string `json:"type"`
}

type PatchDeviceOp struct {
	Op string `json:"op"`
	Path string `json:"path"`
	Value interface{} `json:"value"`
}

func (gatewayRegistryService *GatewayRegistryService) GetGateway(deviceID string) (GatewayImport, error) {
	url := gatewayRegistryService.ServiceURI + "/v3alpha/gateways?relay_id_eq=" + deviceID
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logging.Log.Warningf("GatewayRegistryService.GetGateway() failed to intialize the http request. Error: %v", err)

		return GatewayImport{}, err
	}

	request.Header.Set("X-WigWag-Identity", "{}")

	client := &http.Client{}

	response, err := client.Do(request)
	if err != nil {
		logging.Log.Warningf("GatewayRegistryService.GetGateway() failed to execute the http GET request. Error: %v", err)

		return GatewayImport{}, err
	}

	defer response.Body.Close()

	if response.StatusCode != 200 {
		logging.Log.Warningf("GatewayRegistryService.GetGateway() Could not query gateway registry http status: %d", response.StatusCode)

		return GatewayImport{}, ErrGatewayQueryFailed
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logging.Log.Warningf("GatewayRegistryService.GetGateway() failed to get the stream of the response body. Error: %v", err)

		return GatewayImport{}, err
	}

	var responseBody GetGatewayResponse
	if err := json.Unmarshal(body, &responseBody); err != nil {
		logging.Log.Warningf("GatewayRegistryService.GetGateway() failed to parse the body of the response into the expected body. Body: %+v, Error: %v", string(body), err)

		return GatewayImport{}, err
	}

	if len(responseBody.Data) == 0 {
		logging.Log.Warningf("GatewayRegistryService.GetGateway() failed to find the gateway. Error: %v", ErrGatewayNotFound)

		return GatewayImport{}, ErrGatewayNotFound
	} else if len(responseBody.Data) > 1 {
		logging.Log.Warningf("GatewayRegistryService.GetGateway() failed to get the expected number of gateway. Error: %v")

		return GatewayImport{}, ErrGatewayUnexpectedFound
	}

	logging.Log.Infof("GatewayRegistryService.GetGateway() retrieve a gateway: %+v", responseBody.Data[0])

	return responseBody.Data[0], nil
}

func (gatewayRegistryService *GatewayRegistryService) GatewaysExists(deviceID string) (bool, error) {
	_, err := gatewayRegistryService.GetGateway(deviceID)

	if err != nil {
		return false, err
	}

	return true, nil
}

func (gatewayRegistryService *GatewayRegistryService) RegistryGateway(deviceID string, accountID string, siteID string) error {
	gateway := GatewayImport{
		IP:        "0.0.0.0",
		AccountID: accountID,
		SiteID:    siteID,
		RelayID:   deviceID,
	}

	gatewayJSON, err := json.Marshal(gateway)
	if err != nil {
		logging.Log.Warningf("GatewayRegistryService.RegistryGateway() failed to parse the gateway object into JSON format. Error: %v", err)

		return err
	}

	url := gatewayRegistryService.ServiceURI + "/v3alpha/gateways"
	request, err := http.NewRequest("POST", url, strings.NewReader(string(gatewayJSON)))
	if err != nil {
		logging.Log.Warningf("GatewayRegistryService.RegistryGateway() failed to intialize the http request. Url: %v, Error: %v.", request.URL.String(), err)

		return err
	}

	request.Header.Set("X-WigWag-Identity", "{}")

	client := &http.Client{}

	response, err := client.Do(request)
	if err != nil {
		logging.Log.Warningf("GatewayRegistryService.RegistryGateway() failed to execute the http POST request. Error: %v", err)

		return err
	}

	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		logging.Log.Warningf("GatewayRegistryService.RegistryGateway() failed to register the gateway to the cloud. HTTP Status Code: %v, Error: %v", response.StatusCode, response.StatusCode)

		return ErrRegistryGatewayFailure
	}

	logging.Log.Debugf("GatewayRegistryService.RegistryGateway() successfully registered the gateway to the cloud. HTTP Status Code: %v, Message: %v", response.StatusCode, response.StatusCode)

	return nil
}

func (gatewayRegistryService *GatewayRegistryService) PatchGateway(deviceID string, ops []PatchDeviceOp) *adapters.MbedError {
	bodyJSON, err := json.Marshal(ops)

	if err != nil {
		logging.Log.Warningf("GatewayRegistryService.PatchGateway() failed to parse the gateway object into JSON format. Error: %v", err)

		return &adapters.MbedError{ Code: http.StatusBadRequest, Type: "bad_request", Message: "Unable to encode request body bound for the gateway registry service" }
	}

	url := gatewayRegistryService.ServiceURI + "/v3alpha/gateways/" + deviceID
	request, err := http.NewRequest("PATCH", url, strings.NewReader(string(bodyJSON)))

	if err != nil {
		logging.Log.Warningf("GatewayRegistryService.PatchGateway() failed to intialize the http request. Url: %v, Error: %v.", request.URL.String(), err)

		return &adapters.MbedError{ Code: http.StatusInternalServerError, Type: "internal_server_error", Message: "Unable to reach gateway registry service: " + err.Error() }
	}

	request.Header.Set("X-WigWag-Identity", "{}")

	client := &http.Client{}

	response, err := client.Do(request)

	if err != nil {
		logging.Log.Warningf("GatewayRegistryService.PatchGateway() failed to execute the http POST request. Error: %v", err)

		return &adapters.MbedError{ Code: http.StatusInternalServerError, Type: "internal_server_error", Message: "Unable to reach gateway registry service: " + err.Error() }
	}

	body, err := ioutil.ReadAll(response.Body)

	if err != nil {
		logging.Log.Warningf("GatewayRegistryService.PatchGateway() failed to get the stream of the response body. Error: %v", err)

		return &adapters.MbedError{ Code: http.StatusInternalServerError, Type: "internal_server_error", Message: "Unable to read response from gateway registry" }
	}

	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		logging.Log.Warningf("GatewayRegistryService.PatchGateway() received error response status = %d body = %s", response.StatusCode, string(body))

		var errorResponse adapters.MbedError

		if err := json.Unmarshal(body, &errorResponse); err != nil {
			return &adapters.MbedError{ Code: http.StatusInternalServerError, Type: "internal_server_error", Message: "Could not parse response from gateway registry" }
		}

		return &errorResponse
	}

	return nil
}