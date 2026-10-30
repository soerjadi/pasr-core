package rest

import (
	"net/http"
	"strconv"
)

// maxParsedLimit bounds the parsed value before services clamp it to the contract maximum (50).
const maxParsedLimit = 1000

// Limit parses the optional `limit` query parameter shared by every list operation. It returns 0
// when absent (the service applies the default); anything but an integer >= 1 is a 400, matching
// `minimum: 1` in docs/api/openapi.yaml. On false the error response is already written.
func Limit(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", "limit must be a positive integer")
		return 0, false
	}
	return min(n, maxParsedLimit), true
}
