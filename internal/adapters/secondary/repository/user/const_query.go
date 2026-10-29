package user

const (
	upsertByPhoneQuery = `
		INSERT INTO users (phone, role)
		VALUES ($1, $2)
		ON CONFLICT (phone) DO UPDATE SET role = EXCLUDED.role
		RETURNING id, phone, name, role
	`
	getByIDQuery = `
		SELECT id, phone, name, role
		FROM users
		WHERE id = $1
	`
)
