package product

import (
	"fmt"
	"strings"

	"github.com/oemah/nusantara/core/internal/core/domain"
)

// VisibleSQL is the single definition of public product visibility: products are only shown
// when the store is verified and the product is neither hidden nor deleted. It expects the
// aliases p (products) and s (stores), so every product query, in any package, can embed it:
//
//	FROM products p JOIN stores s ON s.id = p.store_id WHERE ` + product.VisibleSQL
const VisibleSQL = `s.status = 'VERIFIED' AND p.status <> 'HIDDEN' AND p.deleted_at IS NULL`

// RatingSQL is the average rating rounded to one decimal, 0 without reviews.
const RatingSQL = `COALESCE(ROUND(p.rating_sum::numeric / NULLIF(p.rating_count, 0), 1), 0)::float8`

const (
	getVisibleQuery = `
		SELECT p.id, p.name, p.store_id, s.name, p.description, p.price, ` + RatingSQL + `,
		       p.rating_count, p.stock, p.halal_verified,
		       ($2::bigint > 0 AND EXISTS (SELECT 1 FROM favorites f WHERE f.user_id = $2 AND f.product_id = p.id))
		FROM products p
		JOIN stores s ON s.id = p.store_id
		WHERE p.id = $1 AND ` + VisibleSQL

	imagesQuery = `
		SELECT pi.product_id, u.keys
		FROM product_images pi
		JOIN uploads u ON u.id = pi.upload_id
		WHERE pi.product_id = ANY($1)
		ORDER BY pi.product_id, pi.position
	`
)

// likeEscaper escapes the LIKE wildcards and the escape character itself.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// buildListQuery assembles the list statement with only the filters that are set, so the
// planner sees plain predicates it can match to the (category_id|store_id|featured, id DESC) indexes.
func buildListQuery(q domain.ProductQuery) (string, []any) {
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	var b strings.Builder
	b.WriteString(`SELECT p.id, p.name, p.store_id, s.name, p.price, ` + RatingSQL + `,
		p.rating_count, p.stock, p.halal_verified
		FROM products p
		JOIN stores s ON s.id = p.store_id
		WHERE ` + VisibleSQL)
	if q.CategoryID > 0 {
		b.WriteString(" AND p.category_id = " + arg(q.CategoryID))
	}
	if q.SellerID > 0 {
		b.WriteString(" AND p.store_id = " + arg(q.SellerID))
	}
	if q.Featured {
		b.WriteString(" AND p.featured")
	}
	if q.Q != "" {
		b.WriteString(` AND lower(p.name) LIKE '%' || lower(` + arg(likeEscaper.Replace(q.Q)) + `) || '%' ESCAPE '\'`)
	}
	if q.BeforeID > 0 {
		b.WriteString(" AND p.id < " + arg(q.BeforeID))
	}
	b.WriteString(" ORDER BY p.id DESC LIMIT " + arg(q.Limit))
	return b.String(), args
}

const (
	createQuery = `
		INSERT INTO products (store_id, category_id, name, description, price, stock, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	ownedColumns = `p.id, p.store_id, p.category_id, p.name, p.price, p.stock, p.description, p.status`

	getOwnedQuery = `SELECT ` + ownedColumns + `
		FROM products p WHERE p.id = $1 AND p.store_id = $2 AND p.deleted_at IS NULL`

	// FOR UPDATE serialises concurrent patches of one product.
	getOwnedForUpdateQuery = getOwnedQuery + ` FOR UPDATE`

	countOwnedQuery = `
		SELECT status, count(*)::int
		FROM products
		WHERE store_id = $1 AND deleted_at IS NULL
		GROUP BY status
	`
	updateQuery = `
		UPDATE products SET name = $2, price = $3, stock = $4, description = $5, status = $6
		WHERE id = $1 AND deleted_at IS NULL
	`
	deleteImagesQuery = `DELETE FROM product_images WHERE product_id = $1`
	insertImagesQuery = `
		INSERT INTO product_images (product_id, upload_id, position)
		SELECT $1, u.id, u.ord - 1 FROM unnest($2::bigint[]) WITH ORDINALITY AS u(id, ord)
	`
	softDeleteQuery      = `UPDATE products SET deleted_at = now() WHERE id = $1 AND store_id = $2 AND deleted_at IS NULL`
	removeFavoritesQuery = `DELETE FROM favorites WHERE product_id = $1`
)

// buildListOwnedQuery lists a store's non-deleted products newest first on the (store_id, id DESC) index.
func buildListOwnedQuery(q domain.SellerProductQuery) (string, []any) {
	args := []any{q.StoreID}
	var b strings.Builder
	b.WriteString(`SELECT ` + ownedColumns + ` FROM products p WHERE p.store_id = $1 AND p.deleted_at IS NULL`)
	if q.Status != "" {
		args = append(args, string(q.Status))
		b.WriteString(fmt.Sprintf(" AND p.status = $%d", len(args)))
	}
	if q.BeforeID > 0 {
		args = append(args, q.BeforeID)
		b.WriteString(fmt.Sprintf(" AND p.id < $%d", len(args)))
	}
	args = append(args, q.Limit)
	b.WriteString(fmt.Sprintf(" ORDER BY p.id DESC LIMIT $%d", len(args)))
	return b.String(), args
}
