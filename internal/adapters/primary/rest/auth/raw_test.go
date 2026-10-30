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
	defer func() { _ = res.Body.Close() }()
	return res.StatusCode
}

// requestRaw posts an OTP request without touching *testing.T, so it is safe in goroutines.
func requestRaw(baseURL, phone, ip string) int {
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/auth/otp/request", strings.NewReader(`{"phone":"`+phone+`"}`))
	if err != nil {
		return -1
	}
	req.Header.Set("Content-Type", "application/json")
	if ip != "" {
		req.Header.Set("CF-Connecting-IP", ip)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1
	}
	defer func() { _ = res.Body.Close() }()
	return res.StatusCode
}
