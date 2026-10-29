package storemgmt

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/storage"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

const (
	maxNameRunes        = 80
	maxDescriptionRunes = 2000
	maxPhotos           = 5
)

type service struct {
	products repository.ProductRepository
	stores   repository.StoreRepository
	uploads  repository.UploadRepository
	storage  storage.ObjectStorage
	tx       repository.TxManager
}

func NewService(
	products repository.ProductRepository,
	stores repository.StoreRepository,
	uploads repository.UploadRepository,
	objects storage.ObjectStorage,
	tx repository.TxManager,
) services.StoreManagementService {
	return &service{products: products, stores: stores, uploads: uploads, storage: objects, tx: tx}
}

func invalid(msg string) error {
	return domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", msg)
}

func notFound() error {
	return domain.NewError(domain.ErrNotFound, "NOT_FOUND", "product not found")
}

// storeOf finds the caller's store or fails with 409 NO_STORE.
func (s *service) storeOf(ctx context.Context, ownerID int64) (domain.StoreRef, error) {
	ref, err := s.stores.GetRefByOwner(ctx, ownerID)
	if err != nil {
		return domain.StoreRef{}, err
	}
	if ref == nil {
		return domain.StoreRef{}, domain.NewError(domain.ErrConflict, "NO_STORE", "you do not have a store yet")
	}
	return *ref, nil
}

func validName(v string) (string, error) {
	v = strings.TrimSpace(v)
	if n := utf8.RuneCountInString(v); n < 1 || n > maxNameRunes {
		return "", invalid("name must be between 1 and " + strconv.Itoa(maxNameRunes) + " characters")
	}
	return v, nil
}

func validPrice(v int64) error {
	if v < 1 {
		return invalid("price must be at least 1")
	}
	return nil
}

func validStock(v int32) error {
	if v < 0 {
		return invalid("stock must not be negative")
	}
	return nil
}

func validDescription(v string) error {
	if utf8.RuneCountInString(v) > maxDescriptionRunes {
		return invalid("description must be at most " + strconv.Itoa(maxDescriptionRunes) + " characters")
	}
	return nil
}

func validPhotos(ids []int64) error {
	if len(ids) > maxPhotos {
		return invalid("at most " + strconv.Itoa(maxPhotos) + " photos are allowed")
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return invalid("photoIds must be distinct upload ids")
		}
		seen[id] = true
	}
	return nil
}

// claimPhotos checks the uploads belong to ownerID and are unattached, marks them attached and
// returns their keys in the order of ids. It must run inside a transaction.
func (s *service) claimPhotos(ctx context.Context, ownerID int64, ids []int64) ([]domain.ImageKeys, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	resolved, err := s.uploads.ResolveOwned(ctx, ownerID, ids)
	if err != nil {
		return nil, err
	}
	keys := make([]domain.ImageKeys, len(ids))
	for i, id := range ids {
		k, ok := resolved[id]
		if !ok {
			return nil, invalid("photoIds contains an unknown or already used upload")
		}
		keys[i] = k
	}
	if err := s.uploads.MarkAttached(ctx, ids); err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *service) imageSet(k domain.ImageKeys) domain.ImageSet {
	return domain.ImageSet{Small: s.storage.URL(k.Small), Medium: s.storage.URL(k.Medium), Large: s.storage.URL(k.Large)}
}

func (s *service) view(r domain.SellerProductRecord, keys []domain.ImageKeys) domain.SellerProduct {
	images := make([]domain.ImageSet, 0, len(keys))
	for _, k := range keys {
		images = append(images, s.imageSet(k))
	}
	return domain.SellerProduct{
		ID: r.ID, Name: r.Name, Price: r.Price, Stock: r.Stock, Description: r.Description, Status: r.Status, Images: images,
	}
}

// load builds the API view of one product, reading its images.
func (s *service) load(ctx context.Context, r domain.SellerProductRecord) (domain.SellerProduct, error) {
	imgs, err := s.products.Images(ctx, []int64{r.ID})
	if err != nil {
		return domain.SellerProduct{}, err
	}
	return s.view(r, imgs[r.ID]), nil
}
