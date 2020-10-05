package token

type ArmAccessToken struct {
	AccountID string `json:"account_id"`
	RequestID string `json:"request_id"`
}

type WigwagIdentityHeader struct {
	ClientType string `json:"clientType"`
	AccountID string `json:"accountID"`
	UserID string `json:"userID,omitempty"`
}