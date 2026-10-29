package repository

import (
	"context"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

type SocialRepository interface {
	// ListFavorites returns the user's favorites that are still publicly visible, newest first
	// (favorite id DESC); beforeID > 0 keeps only favorites with a smaller id. limit rows are fetched.
	ListFavorites(ctx context.Context, userID, beforeID int64, limit int) ([]domain.FavoriteRecord, error)
	// AddFavorite is idempotent: an existing favorite is left as it is.
	AddFavorite(ctx context.Context, userID, productID int64) error
	// RemoveFavorite is idempotent: removing a missing favorite succeeds.
	RemoveFavorite(ctx context.Context, userID, productID int64) error
	// Follow is idempotent: an existing follow is left as it is.
	Follow(ctx context.Context, userID, storeID int64) error
	// Unfollow is idempotent: unfollowing a store that is not followed succeeds.
	Unfollow(ctx context.Context, userID, storeID int64) error
	// OwnsStore reports whether the store belongs to the user.
	OwnsStore(ctx context.Context, userID, storeID int64) (bool, error)
}
