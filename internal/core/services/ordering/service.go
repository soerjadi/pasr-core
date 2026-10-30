package ordering

import (
	"time"

	"github.com/oemah/nusantara/core/internal/core/domain"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/repository"
	"github.com/oemah/nusantara/core/internal/core/ports/secondary/storage"
	"github.com/oemah/nusantara/core/internal/core/ports/services"
)

// MaxCartLines is the most distinct lines a cart request may carry.
const MaxCartLines = 50

// Config holds the ordering rules taken from configuration.
type Config struct {
	// AcceptWindow is how long a seller has to accept a new seller-order (orders.accept_window_minutes).
	AcceptWindow time.Duration
	// Location is the business time zone (WIB) that dates order numbers.
	Location *time.Location
}

type service struct {
	carts   repository.CartRepository
	orders  repository.OrderRepository
	keys    repository.IdempotencyRepository
	tx      repository.TxManager
	notify  services.NotifyService
	storage storage.ObjectStorage
	clock   domain.Clock
	cfg     Config
}

func NewService(
	carts repository.CartRepository,
	orders repository.OrderRepository,
	keys repository.IdempotencyRepository,
	tx repository.TxManager,
	notify services.NotifyService,
	objects storage.ObjectStorage,
	clock domain.Clock,
	cfg Config,
) services.OrderingService {
	if cfg.Location == nil {
		cfg.Location = time.UTC
	}
	return &service{carts: carts, orders: orders, keys: keys, tx: tx, notify: notify, storage: objects, clock: clock, cfg: cfg}
}

func invalid(msg string) error {
	return domain.NewError(domain.ErrValidationFailed, "VALIDATION_FAILED", msg)
}

func (s *service) imageSet(k domain.ImageKeys) domain.ImageSet {
	return domain.ImageSet{Small: s.storage.URL(k.Small), Medium: s.storage.URL(k.Medium), Large: s.storage.URL(k.Large)}
}

// present turns the stored image keys of every item into URLs.
func (s *service) present(o *domain.Order) {
	for i := range o.SellerOrders {
		items := o.SellerOrders[i].Items
		for j := range items {
			if items[j].ImageKeys != nil {
				items[j].Image = s.imageSet(*items[j].ImageKeys)
			}
		}
	}
}
