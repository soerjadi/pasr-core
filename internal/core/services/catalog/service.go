package catalog

import (
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/storage"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

// Config holds the catalog limits.
type Config struct {
	MaxImageBytes  int64         // largest accepted upload body
	MaxImagePixels int64         // largest accepted width*height
	OrphanAge      time.Duration // unattached uploads older than this are purged; default 24h
}

// maxConcurrentImages bounds simultaneous decodes: one 40 MP image needs ~160 MB of pixels.
const maxConcurrentImages = 2

type service struct {
	categories repository.CategoryRepository
	products   repository.ProductRepository
	storefront repository.StorefrontRepository
	reviews    repository.ReviewRepository
	uploads    repository.UploadRepository
	storage    storage.ObjectStorage
	cfg        Config
	clock      domain.Clock
	slots      chan struct{}
}

func NewService(
	categories repository.CategoryRepository,
	products repository.ProductRepository,
	storefront repository.StorefrontRepository,
	reviews repository.ReviewRepository,
	uploads repository.UploadRepository, store storage.ObjectStorage,
	cfg Config,
	clock domain.Clock,
) services.CatalogService {
	if cfg.OrphanAge <= 0 {
		cfg.OrphanAge = 24 * time.Hour
	}
	return &service{
		categories: categories, products: products, storefront: storefront, reviews: reviews, uploads: uploads, storage: store, cfg: cfg, clock: clock,
		slots: make(chan struct{}, maxConcurrentImages),
	}
}
