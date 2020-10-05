package httputil

// PublicError would be returned when an error is returned by a http response
type PublicError struct {
	Object    string  `json:"object"`
	Code      int     `json:"code"`
	Type      string  `json:"type"`
	Message   string  `json:"message"`
	Fields    []PublicErrorField `json:"fields,omitempty"`
	RequestID string  `json:"request_id"`
}

type PublicErrorField struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}