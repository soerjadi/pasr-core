package review

const listByStoreQuery = `
	SELECT r.id, r.rating, r.text, COALESCE(NULLIF(btrim(u.name), ''), 'Pembeli'), r.created_at
	FROM reviews r
	JOIN users u ON u.id = r.buyer_id
	WHERE r.store_id = $1 AND ($2::bigint = 0 OR r.id < $2)
	ORDER BY r.id DESC
	LIMIT $3
`
