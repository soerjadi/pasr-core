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
	var coded *domain.CodedError
	if errors.As(err, &coded) {
		if status := kindStatus(coded.Kind); status != 0 {
			WriteError(w, status, coded.Code, coded.Message)
			return
		}
	}
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
	case errors.Is(err, domain.ErrUnprocessable):
		WriteError(w, http.StatusUnprocessableEntity, "UNPROCESSABLE", err.Error())
	case errors.Is(err, domain.ErrTooManyRequests):
		WriteError(w, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", err.Error())
	default:
		log.ErrorCtx(r.Context(), "unhandled service error", log.Field("error", err), log.Field("path", r.URL.Path))
		WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal server error")
	}
}

func kindStatus(kind error) int {
	switch {
	case errors.Is(kind, domain.ErrValidationFailed):
		return http.StatusBadRequest
	case errors.Is(kind, domain.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(kind, domain.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(kind, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(kind, domain.ErrConflict):
		return http.StatusConflict
	case errors.Is(kind, domain.ErrUnprocessable):
		return http.StatusUnprocessableEntity
	case errors.Is(kind, domain.ErrTooManyRequests):
		return http.StatusTooManyRequests
	}
	return 0
}
