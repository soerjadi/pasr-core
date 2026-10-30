package cart

const loadProductsQuery = `
	SELECT p.id, p.name, p.price, p.stock, p.status, p.deleted_at IS NOT NULL, pu.keys,
	       s.id, s.name, su.keys, s.kelurahan,
	       COALESCE(ROUND(s.rating_sum::numeric / NULLIF(s.rating_count, 0), 1), 0)::float8,
	       s.rating_count, s.address, s.hours, s.whatsapp, s.status, s.is_open, s.owner_id
	FROM products p
	JOIN stores s ON s.id = p.store_id
	LEFT JOIN uploads su ON su.id = s.photo_upload_id
	LEFT JOIN LATERAL (
		SELECT u.keys FROM product_images pi JOIN uploads u ON u.id = pi.upload_id
		WHERE pi.product_id = p.id ORDER BY pi.position LIMIT 1
	) pu ON true
	WHERE p.id = ANY($1)
	ORDER BY p.id
`

// FOR UPDATE OF p locks the product rows only: stores are read, never locked.
const loadProductsForUpdateQuery = loadProductsQuery + ` FOR UPDATE OF p`
