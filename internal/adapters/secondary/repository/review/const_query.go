package review

const listByStoreQuery = `
	SELECT r.id, r.rating, r.text, COALESCE(NULLIF(btrim(u.name), ''), 'Pembeli'), r.created_at
	FROM reviews r
	JOIN users u ON u.id = r.buyer_id
	WHERE r.store_id = $1 AND ($2::bigint = 0 OR r.id < $2)
	ORDER BY r.id DESC
	LIMIT $3
`

const createQuery = `
	WITH ins AS (
		INSERT INTO reviews (order_item_id, product_id, store_id, buyer_id, rating, text)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, rating, text, buyer_id, created_at
	)
	SELECT ins.id, ins.rating, ins.text, COALESCE(NULLIF(btrim(u.name), ''), 'Pembeli'), ins.created_at
	FROM ins
	JOIN users u ON u.id = ins.buyer_id
`

// addRatingQuery updates both aggregates in one statement.
const addRatingQuery = `
	WITH p AS (
		UPDATE products SET rating_sum = rating_sum + $3, rating_count = rating_count + 1 WHERE id = $1 RETURNING id
	), s AS (
		UPDATE stores SET rating_sum = rating_sum + $3, rating_count = rating_count + 1 WHERE id = $2 RETURNING id
	)
	SELECT (SELECT count(*) FROM p) + (SELECT count(*) FROM s)
`

const deleteQuery = `DELETE FROM reviews WHERE id = $1 RETURNING product_id, store_id, rating`

// subtractRatingQuery takes a removed review back out of both aggregates in one statement. GREATEST
// keeps aggregates that are already out of step with the reviews from going negative.
const subtractRatingQuery = `
	WITH p AS (
		UPDATE products SET rating_sum = GREATEST(rating_sum - $3, 0), rating_count = GREATEST(rating_count - 1, 0)
		WHERE id = $1 RETURNING id
	), s AS (
		UPDATE stores SET rating_sum = GREATEST(rating_sum - $3, 0), rating_count = GREATEST(rating_count - 1, 0)
		WHERE id = $2 RETURNING id
	)
	SELECT (SELECT count(*) FROM p) + (SELECT count(*) FROM s)
`
