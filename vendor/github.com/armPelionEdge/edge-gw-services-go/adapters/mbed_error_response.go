package adapters

import (
	"encoding/json"
	"net/http"
)

type MbedError struct {
	Code int `json:"code"`
	RequestID string `json:"request_id"`
	Object string `json:"object"`
	Message string `json:"message"`
	Fields []MbedErrorField `json:"fields,omitempty"`
	Type string `json:"type"`
}

type MbedErrorField struct {
	Name string `json:"name"`
	Message string `json:"message"`
}

func MbedErrorResponse(w http.ResponseWriter, r *http.Request, err MbedError) {
	err.RequestID = r.Header.Get("X-Request-ID")
	err.Object = "error"

	body, _ := json.Marshal(err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Code)
	w.Write(body)
}