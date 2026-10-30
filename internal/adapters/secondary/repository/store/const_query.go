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
	getRefByOwnerQuery = `SELECT id, category_id FROM stores WHERE owner_id = $1`
	findByOwnerQuery   = `
		SELECT s.id, s.name, s.category_id, s.owner_name, s.kelurahan, s.address, s.description,
		       s.photo_upload_id, u.keys, s.status, s.is_open, s.whatsapp, s.hours
		FROM stores s
		LEFT JOIN uploads u ON u.id = s.photo_upload_id
		WHERE s.owner_id = $1
	`
	// One statement with a COALESCE per field: a patch only touches the fields it provides, and
	// the row lock Postgres takes serialises concurrent patches without losing either.
	updateQuery = `
		UPDATE stores SET
			name            = COALESCE($2, name),
			category_id     = COALESCE($3, category_id),
			owner_name      = COALESCE($4, owner_name),
			kelurahan       = COALESCE($5, kelurahan),
			address         = COALESCE($6, address),
			description     = COALESCE($7, description),
			photo_upload_id = COALESCE($8, photo_upload_id),
			whatsapp        = COALESCE($9, whatsapp),
			hours           = COALESCE($10::jsonb, hours)
		WHERE id = $1
	`
	setOpenQuery = `UPDATE stores SET is_open = $2 WHERE id = $1`
)

const adminColumns = `
	SELECT id, owner_id, name, owner_name, category_id, kelurahan, address, whatsapp, status, is_open, created_at
	FROM stores
`

const (
	// listAdminQuery pages newest first on the primary key; stores are few (one per seller), so the
	// status filter needs no index of its own beyond stores_status_idx.
	listAdminQuery = adminColumns + `
		WHERE ($1 = '' OR status = $1) AND ($2::bigint = 0 OR id < $2)
		ORDER BY id DESC
		LIMIT $3
	`
	getAdminQuery          = adminColumns + `WHERE id = $1`
	getAdminForUpdateQuery = getAdminQuery + ` FOR UPDATE`
	setStatusQuery         = `UPDATE stores SET status = $2 WHERE id = $1`
)
