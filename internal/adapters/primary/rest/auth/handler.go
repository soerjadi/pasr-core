package auth

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

const maxBodyBytes = 4 << 10

type Handler struct {
	identity services.IdentityService
}

func NewHandler(identity services.IdentityService) *Handler {
	return &Handler{identity: identity}
}

type otpRequest struct {
	Phone string `json:"phone"`
}

type otpVerify struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *Handler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	var req otpRequest
	if !decode(w, r, &req) {
		return
	}
	if err := h.identity.RequestOTP(r.Context(), req.Phone, clientIP(r)); err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req otpVerify
	if !decode(w, r, &req) {
		return
	}
	s, err := h.identity.VerifyOTP(r.Context(), req.Phone, req.Code)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, s)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decode(w, r, &req) {
		return
	}
	s, err := h.identity.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, s)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decode(w, r, &req) {
		return
	}
	if err := h.identity.Logout(r.Context(), req.RefreshToken); err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		rest.WriteServiceError(w, r, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "invalid request body"))
		return false
	}
	return true
}

// clientIP is CF-Connecting-IP when it holds a valid address, else the connection address without its port.
func clientIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); v != "" {
		if a, err := netip.ParseAddr(v); err == nil {
			return a.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
