package store

const (
	createQuery = `
		INSERT INTO stores (owner_id, name, category_id, owner_name, kelurahan, address, whatsapp, photo_upload_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, name, status
	`
	getSummaryByOwnerQuery = `
		SELECT s.id, s.name, s.status, o.cnt, o.earliest
		FROM stores s
		CROSS JOIN LATERAL (
			SELECT count(*)::int AS cnt, min(accept_by) AS earliest
			FROM seller_orders
			WHERE store_id = s.id AND status = 'CREATED'
		) o
		WHERE s.owner_id = $1
	`
)
