package social

import "github.com/oemah/nusantara/core/internal/adapters/secondary/repository/product"

const (
	// listFavoritesQuery joins on the shared visibility fragment, so hidden, deleted and
	// unverified-store products drop out of the list while their favorite rows stay.
	listFavoritesQuery = `
		SELECT f.id, p.id, p.name, p.store_id, s.name, p.price, ` + product.RatingSQL + `,
		       p.rating_count, p.stock, p.halal_verified
		FROM favorites f
		JOIN products p ON p.id = f.product_id
		JOIN stores s ON s.id = p.store_id
		WHERE f.user_id = $1 AND ($2::bigint = 0 OR f.id < $2) AND ` + product.VisibleSQL + `
		ORDER BY f.id DESC
		LIMIT $3
	`

	addFavoriteQuery    = `INSERT INTO favorites (user_id, product_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	removeFavoriteQuery = `DELETE FROM favorites WHERE user_id = $1 AND product_id = $2`
	followQuery         = `INSERT INTO follows (user_id, store_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	unfollowQuery       = `DELETE FROM follows WHERE user_id = $1 AND store_id = $2`
	ownsStoreQuery      = `SELECT EXISTS (SELECT 1 FROM stores WHERE id = $2 AND owner_id = $1)`
)
