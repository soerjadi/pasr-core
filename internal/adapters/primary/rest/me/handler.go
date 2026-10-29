// Package me handles the signed-in user's profile, push devices and store creation.
package me

import (
	"encoding/json"
	"net/http"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

const maxBodyBytes = 16 << 10

type Handler struct {
	account services.AccountService
}

func NewHandler(account services.AccountService) *Handler {
	return &Handler{account: account}
}

// Every method must run behind the Auth middleware.

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	u, err := h.account.GetMe(r.Context(), p.UserID)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	u, err := h.account.UpdateMe(r.Context(), p.UserID, req.Name)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusOK, u)
}

func (h *Handler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	var req struct {
		FCMToken string `json:"fcmToken"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := h.account.RegisterDevice(r.Context(), p.UserID, req.FCMToken); err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateStore(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	var req domain.CreateStoreInput
	if !decode(w, r, &req) {
		return
	}
	st, err := h.account.CreateStore(r.Context(), p.UserID, req)
	if err != nil {
		rest.WriteServiceError(w, r, err)
		return
	}
	rest.WriteJSON(w, http.StatusCreated, st)
}

func principal(w http.ResponseWriter, r *http.Request) (domain.Claims, bool) {
	p, ok := rest.Principal(r.Context())
	if !ok {
		rest.WriteServiceError(w, r, domain.ErrUnauthorized)
	}
	return p, ok
}

func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		rest.WriteServiceError(w, r, domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", "invalid request body"))
		return false
	}
	return true
}
