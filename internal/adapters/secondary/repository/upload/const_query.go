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
)
