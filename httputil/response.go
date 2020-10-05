package httputil

import "net/http"

const minSuccessStatusCode = 200
const maxSuccessStatusCode = 299

// IsSuccessResponse returns true if the
// status code in the HTTP response indicates
// success (status_code E [200, 299])
func IsSuccessResponse(resp *http.Response) bool {
	return 200 >= minSuccessStatusCode && resp.StatusCode <= maxSuccessStatusCode
}
