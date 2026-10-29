// Package media handles image uploads and, with the local storage driver, serves stored files.
package media

import (
	"errors"
	"net/http"

	"github.com/oemah/nusantara/core/internal/adapters/primary/rest"
	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

// multipartOverhead is allowed on top of the image size for boundaries and part headers.
const multipartOverhead = 64 << 10

// FileServer serves stored objects by key; the local storage adapter implements it.
type FileServer interface {
	Serve(w http.ResponseWriter, r *http.Request, key string)
}

type Handler struct {
	catalog  services.CatalogService
	maxBytes int64
	files    FileServer
}

// NewHandler creates the handler. files may be nil when objects are served by a CDN.
func NewHandler(catalog services.CatalogService, maxImageBytes int64, files FileServer) *Handler {
	return &Handler{catalog: catalog, maxBytes: maxImageBytes, files: files}
}

// HasFiles reports whether the handler serves media itself.
func (h *Handler) HasFiles() bool { return h.files != nil }

var (
	errMissingFile = domain.NewError(domain.ErrValidationFailed, "IMAGE_INVALID", "multipart field \"file\" with an image is required")
	errTooLarge    = domain.NewError(domain.ErrValidationFailed, "IMAGE_TOO_LARGE", "image file is too large")
)

// Upload handles POST /v1/me/images. It must run behind the Auth middleware.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	p, ok := rest.Principal(r.Context())
	if !ok {
		rest.WriteServiceError(w, r, domain.ErrUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes+multipartOverhead)

	// Stream the multipart body instead of ParseMultipartForm so nothing is spooled to temp files.
	mr, err := r.MultipartReader()
	if err != nil {
		rest.WriteServiceError(w, r, errMissingFile)
		return
	}
	for {
		part, err := mr.NextPart()
		if err != nil {
			rest.WriteServiceError(w, r, partError(err))
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			continue
		}
		// The part's filename and Content-Type are never used.
		up, err := h.catalog.UploadImage(r.Context(), p.UserID, part)
		if err != nil {
			rest.WriteServiceError(w, r, err)
			return
		}
		rest.WriteJSON(w, http.StatusCreated, up)
		return
	}
}

func partError(err error) error {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return errTooLarge
	}
	return errMissingFile // io.EOF (no "file" part) or a malformed body
}

// Serve handles GET /media/{key...} for the local storage driver.
func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	if h.files == nil {
		http.NotFound(w, r)
		return
	}
	h.files.Serve(w, r, r.PathValue("key"))
}
