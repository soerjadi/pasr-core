package services

import (
	"context"
	"io"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type CatalogService interface {
	ListCategories(ctx context.Context) ([]domain.Category, error)
	// UploadImage validates r as a JPEG, PNG or WebP image, stores three JPEG variants and
	// records them as an unattached upload owned by ownerID.
	UploadImage(ctx context.Context, ownerID int64, r io.Reader) (domain.ImageUpload, error)
	// PurgeOrphanUploads deletes unattached uploads older than the retention period from the
	// database and storage, returning how many uploads were removed.
	PurgeOrphanUploads(ctx context.Context) (int, error)
}
