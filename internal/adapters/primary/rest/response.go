package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	log "github.com/oemahdev/logger"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// ErrorResponse matches the Error schema in docs/api/openapi.yaml.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{Code: code, Message: message})
}

// WriteServiceError maps domain errors to HTTP statuses; anything else is logged and returned as 500.
func WriteServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrValidationFailed):
		WriteError(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
	case errors.Is(err, domain.ErrForbidden):
		WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
	case errors.Is(err, domain.ErrNotFound):
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "not found")
	case errors.Is(err, domain.ErrConflict):
		WriteError(w, http.StatusConflict, "CONFLICT", err.Error())
	default:
		log.ErrorCtx(r.Context(), "unhandled service error", log.Field("error", err), log.Field("path", r.URL.Path))
		WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal server error")
	}
}
