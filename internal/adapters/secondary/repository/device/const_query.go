package device

const upsertQuery = `
	INSERT INTO devices (user_id, fcm_token)
	VALUES ($1, $2)
	ON CONFLICT (fcm_token) DO UPDATE SET user_id = EXCLUDED.user_id, updated_at = now()
`
