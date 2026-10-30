package upload

const (
	createQuery = `
		INSERT INTO uploads (owner_id, keys)
		VALUES ($1, $2)
		RETURNING id
	`
	// FOR UPDATE keeps the orphan GC from deleting rows between resolving and attaching inside a transaction.
	resolveOwnedQuery = `
		SELECT id, keys
		FROM uploads
		WHERE owner_id = $1 AND attached = false AND id = ANY($2)
		FOR UPDATE
	`
	markAttachedQuery = `
		UPDATE uploads SET attached = true
		WHERE id = ANY($1)
	`
	deleteOrphansQuery = `
		DELETE FROM uploads
		WHERE attached = false AND created_at < $1
		RETURNING keys
	`
	// deleteDetachedQuery removes attached uploads nothing refers to any more: replaced or removed
	// product photos and store photos. An attached upload can never be claimed again (resolveOwnedQuery
	// needs attached = false), so once unreferenced it stays unreferenced. Every place that refers to an
	// upload is checked: product_images.upload_id and stores.photo_upload_id (foreign keys, so a missed
	// reference fails the statement instead of deleting), and the order_items.image key snapshots, which
	// keep order history images alive after the product photo changed.
	deleteDetachedQuery = `
		WITH snapshot_keys AS (
			SELECT image->>'small' AS k FROM order_items WHERE image IS NOT NULL
			UNION SELECT image->>'medium' FROM order_items WHERE image IS NOT NULL
			UNION SELECT image->>'large' FROM order_items WHERE image IS NOT NULL
		)
		DELETE FROM uploads u
		WHERE u.attached = true AND u.created_at < $1
		  AND NOT EXISTS (SELECT 1 FROM product_images pi WHERE pi.upload_id = u.id)
		  AND NOT EXISTS (SELECT 1 FROM stores s WHERE s.photo_upload_id = u.id)
		  AND NOT EXISTS (
			SELECT 1 FROM snapshot_keys sk
			WHERE sk.k IN (u.keys->>'small', u.keys->>'medium', u.keys->>'large')
		  )
		RETURNING keys
	`
)
