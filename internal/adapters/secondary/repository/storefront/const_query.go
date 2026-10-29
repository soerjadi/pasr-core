package storefront

const getVisibleQuery = `
	SELECT s.id, s.name, u.keys, s.kelurahan,
	       COALESCE(ROUND(s.rating_sum::numeric / NULLIF(s.rating_count, 0), 1), 0)::float8,
	       s.rating_count, s.address, s.hours, s.whatsapp, s.description,
	       (SELECT count(*) FROM follows f WHERE f.store_id = s.id)::int,
	       ($2::bigint > 0 AND EXISTS (SELECT 1 FROM follows f WHERE f.store_id = s.id AND f.user_id = $2))
	FROM stores s
	LEFT JOIN uploads u ON u.id = s.photo_upload_id
	WHERE s.id = $1 AND s.status = 'VERIFIED'
`
