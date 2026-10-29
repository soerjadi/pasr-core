package category

const (
	getAllQuery = `
		SELECT id, name, icon_key
		FROM categories
		ORDER BY sort_order ASC, id ASC
	`
)
