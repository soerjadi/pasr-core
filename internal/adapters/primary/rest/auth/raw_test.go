package auth_test

import (
	"net/http"
	"strings"
)

// verifyRaw posts a verify request without touching *testing.T, so it is safe in goroutines.
func verifyRaw(baseURL, phone, code string) int {
	body := `{"phone":"` + phone + `","code":"` + code + `"}`
	res, err := http.Post(baseURL+"/v1/auth/otp/verify", "application/json", strings.NewReader(body))
	if err != nil {
		return -1
	}
	defer res.Body.Close()
	return res.StatusCode
}
