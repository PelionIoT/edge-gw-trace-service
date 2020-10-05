package api

type WigwagAPI interface {
	Account(accountID string) Account
	Relay(relayID string) Relay
}

type Account interface {
	Id() string
	Sites() Sites
	Site(siteID string) Site
	Relay(relayID string) Relay
}

type Sites interface {
	Post() (Site, error)
	Get() ([]Site, error)
}

type Site interface {
	Id() string
	Put() error
}

type Relay interface {
	Id() string
	Get() (RelayStatus, error)
	Put(pairingCode string) error
	Patch(pairingCode string, relayPatch RelayPatch) error
}

type RelayStatus struct {
	SiteID string `json:"siteID"`
}

type RelayPatch struct {
	SiteID string `json:"siteID"`
	AccountID string `json:"accountID"`
}