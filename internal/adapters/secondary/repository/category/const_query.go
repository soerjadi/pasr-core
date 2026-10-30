package category

const (
	getAllQuery = `
		SELECT id, name, icon_key
		FROM categories
		ORDER BY sort_order ASC, id ASC
	`
	// createQuery places a category without a sort order after the last one.
	createQuery = `
		INSERT INTO categories (name, icon_key, sort_order)
		VALUES ($1, $2, COALESCE($3, (SELECT COALESCE(max(sort_order), 0) + 1 FROM categories)))
		RETURNING id, name, icon_key
	`
	updateQuery = `
		UPDATE categories SET
			name       = COALESCE($2, name),
			icon_key   = COALESCE($3, icon_key),
			sort_order = COALESCE($4, sort_order)
		WHERE id = $1
		RETURNING id, name, icon_key
	`
	// deleteQuery relies on the stores and products foreign keys (deleted products included) to
	// refuse a category still in use, so the check cannot race a concurrent insert.
	deleteQuery = `DELETE FROM categories WHERE id = $1`
)
