package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/armPelionEdge/wigwag-go-logger/logging"
	"github.com/armPelionEdge/edge-gw-services-go/middleware/wigwag"
	"net/http"
	"net/url"
)

type WigwagAPIImpl struct {
	URL *url.URL
}

func (wigwagAPI *WigwagAPIImpl) Account(accountID string) Account {
	return &AccountImpl{
		url: wigwagAPI.URL,
		accountID: accountID,
	}
}

func (wigwagAPI *WigwagAPIImpl) Relay(relayID string) Relay {
	return &RelayImpl{
		url: wigwagAPI.URL,
		relayID: relayID,
	}
}

type AccountImpl struct {
	accountID string
	url *url.URL
}

func (account *AccountImpl) Id() string {
	return account.accountID
}

func (account *AccountImpl) Relay(relayID string) Relay {
	return &RelayImpl{
		url: account.url,
		accountID: account.accountID,
		relayID: relayID,
	}
}

func (account *AccountImpl) Sites() Sites {
	return &SitesImpl{
		url: account.url,
		accountID: account.accountID,
	}
}

func (account *AccountImpl) Site(siteID string) Site {
	return &SiteImpl{
		url: account.url,
		accountID: account.accountID,
		siteID: siteID,
	}
}

type SitesImpl struct {
	accountID string
	url *url.URL
}

func (sites *SitesImpl) responseToSites(responses []map[string]interface{}) ([]Site, error) {
	var result []Site = make([]Site, 0, len(responses))

	for _, response := range responses {
		if _, ok := response["id"]; !ok {
			logging.Log.Errorf("Error creating site: Response did not contain expected fields: %v", response)

			return nil, APIError{ Status: http.StatusBadRequest }
		}

		strResponse, ok := response["id"].(string)

		if !ok {
			logging.Log.Errorf("Error creating site: Response contained an id field that was not a string: %v", response)

			return nil, APIError{ Status: http.StatusBadRequest }
		}

		result = append(result, &SiteImpl{ url: sites.url, accountID: sites.accountID, siteID: strResponse })
	}

	return result, nil
}

func (sites *SitesImpl) Post() (Site, error) {
	relativePath, err := url.Parse(fmt.Sprintf("./api/v1.0/accounts/%s/sites", sites.accountID))

	if err != nil {
		logging.Log.Criticalf("Error creating relative path: %v", err)

		return nil, err
	}

	request, err := http.NewRequest("POST", sites.url.ResolveReference(relativePath).String(), nil)

	if err != nil {
		logging.Log.Criticalf("Error creating POST request: %v", err)

		return nil, err
	}

	accessHeaderWriter := &wigwag.WigwagAccessHeaderWriterImpl{}
	accessHeaderWriter.WriteAccessHeaders(sites.accountID, request)

	response, err := http.DefaultClient.Do(request)

	if err != nil {
		logging.Log.Errorf("Error creating site: %v", err)

		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		return nil, APIError{ Status: response.StatusCode }
	}

	var responseBody map[string]interface{}
	var decoder *json.Decoder = json.NewDecoder(response.Body)

	if err := decoder.Decode(&responseBody); err != nil {
		logging.Log.Errorf("Error creating site: Unable to decode response: %v", err)

		return nil, err
	}

	result, err := sites.responseToSites([]map[string]interface{}{ responseBody })

	if err != nil {
		return nil, err
	}

	return result[0], nil
}

func (sites *SitesImpl) Get() ([]Site, error) {
	relativePath, err := url.Parse(fmt.Sprintf("./api/v1.0/accounts/%s/sites", sites.accountID))

	if err != nil {
		logging.Log.Criticalf("Error creating relative path: %v", err)

		return nil, err
	}

	request, err := http.NewRequest("GET", sites.url.ResolveReference(relativePath).String(), nil)

	if err != nil {
		logging.Log.Criticalf("Error creating GET request: %v", err)

		return nil, err
	}

	accessHeaderWriter := &wigwag.WigwagAccessHeaderWriterImpl{}
	accessHeaderWriter.WriteAccessHeaders(sites.accountID, request)

	response, err := http.DefaultClient.Do(request)

	if err != nil {
		logging.Log.Errorf("Error listing sites: %v", err)

		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, APIError{ Status: response.StatusCode }
	}

	var responseBody map[string]interface{}
	var decoder *json.Decoder = json.NewDecoder(response.Body)

	if err := decoder.Decode(&responseBody); err != nil {
		logging.Log.Errorf("Error listing sites: Unable to decode response: %v", err)

		return nil, err
	}

	embedded, ok := responseBody["_embedded"]

	if !ok {
		logging.Log.Errorf("Error listing sites: Response did not contain an _embedded field")

		return nil, APIError{ Status: http.StatusBadRequest }
	}

	embeddedMap, ok := embedded.(map[string]interface{})

	if !ok {
		logging.Log.Errorf("Error listing sites: Response contained an _embedded field but it was not an object")

		return nil, APIError{ Status: http.StatusBadRequest }
	}

	embeddedSites, ok := embeddedMap["sites"]

	if !ok {
		logging.Log.Errorf("Error listing sites: Response contained an _embedded field but it did not contain a sites property")

		return nil, APIError{ Status: http.StatusBadRequest }
	}

	sitesList, ok := embeddedSites.([]map[string]interface{})

	if !ok {
		logging.Log.Errorf("Error listing sites: Response contained an _embedded.sites field but it was not a list")

		return nil, APIError{ Status: http.StatusBadRequest }
	}

	return sites.responseToSites(sitesList)
}

type SiteImpl struct {
	accountID string
	siteID string
	url *url.URL
}

func (site *SiteImpl) Id() string {
	return site.siteID
} 

func (site *SiteImpl) Put() error {
	if site.accountID == "" {
		logging.Log.Criticalf("Attempted to call WigwagAPI.Site().Put()")

		return ENoAccountId
	}

	relativePath, err := url.Parse(fmt.Sprintf("./api/v1.0/accounts/%s/sites/%s", site.accountID, site.siteID))

	if err != nil {
		logging.Log.Criticalf("Error creating relative path: %v", err)

		return err
	}

	finalURL := site.url.ResolveReference(relativePath)

	request, err := http.NewRequest("PUT", finalURL.String(), nil)

	if err != nil {
		logging.Log.Criticalf("Error creating PUT request: %v", err)

		return err
	}

	accessHeaderWriter := &wigwag.WigwagAccessHeaderWriterImpl{}
	accessHeaderWriter.WriteAccessHeaders(site.accountID, request)

	response, err := http.DefaultClient.Do(request)

	if err != nil {
		logging.Log.Errorf("Error creating site: %v", err)

		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return APIError{ Status: response.StatusCode }
	}

	return nil
}

type RelayImpl struct {
	accountID string
	relayID string
	url *url.URL
}

func (relay *RelayImpl) Id() string {
	return relay.relayID
}

func (relay *RelayImpl) Put(pairingCode string) error {
	if relay.accountID == "" {
		logging.Log.Criticalf("Attempted to call WigwagAPI.Relay().Put()")

		return ENoAccountId
	}

	relativePath, err := url.Parse(fmt.Sprintf("./api/v1.0/accounts/%s/relays/%s", relay.accountID, relay.relayID))

	if err != nil {
		logging.Log.Criticalf("Error creating relative path: %v", err)

		return err
	}

	q := url.Values{}
	q.Set("pairingCode", pairingCode)

	finalURL := relay.url.ResolveReference(relativePath)
	finalURL.RawQuery = q.Encode()

	request, err := http.NewRequest("PUT", finalURL.String(), nil)

	if err != nil {
		logging.Log.Criticalf("Error creating PUT request: %v", err)

		return err
	}

	accessHeaderWriter := &wigwag.WigwagAccessHeaderWriterImpl{}
	accessHeaderWriter.WriteAccessHeaders(relay.accountID, request)

	response, err := http.DefaultClient.Do(request)

	if err != nil {
		logging.Log.Errorf("Error importing relay: %v", err)

		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return APIError{ Status: response.StatusCode }
	}

	return nil
}

func (relay *RelayImpl) Patch(pairingCode string, patch RelayPatch) error {
	if relay.accountID == "" {
		logging.Log.Criticalf("Attempted to call WigwagAPI.Relay().Patch()")

		return ENoAccountId
	}

	relativePath, err := url.Parse(fmt.Sprintf("./api/v1.0/accounts/%s/relays/%s", relay.accountID, relay.relayID))

	if err != nil {
		logging.Log.Criticalf("Error creating relative path: %v", err)

		return err
	}

	body, err := json.Marshal(patch)

	if err != nil {
		logging.Log.Criticalf("Error encoding patch body: %v", err)

		return err
	}

	logging.Log.Debugf("Patch body: %v", string(body))

	q := url.Values{}
	q.Set("pairingCode", pairingCode)

	finalURL := relay.url.ResolveReference(relativePath)
	finalURL.RawQuery = q.Encode()

	request, err := http.NewRequest("PATCH", finalURL.String(), bytes.NewReader(body))

	if err != nil {
		logging.Log.Criticalf("Error creating PATCH request: %v", err)

		return err
	}

	accessHeaderWriter := &wigwag.WigwagAccessHeaderWriterImpl{}
	accessHeaderWriter.WriteAccessHeaders(relay.accountID, request)
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)

	if err != nil {
		logging.Log.Errorf("Error binding relay: %v", err)

		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return APIError{ Status: response.StatusCode }
	}

	return nil
}

func (relay *RelayImpl) Get() (RelayStatus, error) {
	if relay.accountID == "" {
		logging.Log.Criticalf("Attempted to call WigwagAPI.Relay().Get()")

		return RelayStatus{}, ENoAccountId
	}

	relativePath, err := url.Parse(fmt.Sprintf("./api/v1.0/accounts/%s/relays/%s", relay.accountID, relay.relayID))

	if err != nil {
		logging.Log.Criticalf("Error creating relative path: %v", err)

		return RelayStatus{}, err
	}

	request, err := http.NewRequest("GET", relay.url.ResolveReference(relativePath).String(), nil)

	if err != nil {
		logging.Log.Criticalf("Error creating GET request: %v", err)

		return RelayStatus{}, err
	}

	accessHeaderWriter := &wigwag.WigwagAccessHeaderWriterImpl{}
	accessHeaderWriter.WriteAccessHeaders(relay.accountID, request)

	response, err := http.DefaultClient.Do(request)

	if err != nil {
		logging.Log.Errorf("Error requesting relay status: %v", err)

		return RelayStatus{}, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return RelayStatus{}, APIError{ Status: response.StatusCode }
	}

	var relayStatus RelayStatus
	var decoder *json.Decoder = json.NewDecoder(response.Body)

	if err := decoder.Decode(&relayStatus); err != nil {
		logging.Log.Errorf("Error requesting relay status: Unable to decode response: %v", err)

		return RelayStatus{}, err
	}

	return relayStatus, nil
}