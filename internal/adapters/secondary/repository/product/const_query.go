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

// ratingSQL is the average rating rounded to one decimal, 0 without reviews.
const ratingSQL = `COALESCE(ROUND(p.rating_sum::numeric / NULLIF(p.rating_count, 0), 1), 0)::float8`

const (
	getVisibleQuery = `
		SELECT p.id, p.name, p.store_id, s.name, p.description, p.price, ` + ratingSQL + `,
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
	b.WriteString(`SELECT p.id, p.name, p.store_id, s.name, p.price, ` + ratingSQL + `,
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
