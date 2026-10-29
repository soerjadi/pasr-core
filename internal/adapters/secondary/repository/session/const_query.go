package session

const (
	createQuery = `
		INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	getByHashQuery = `
		SELECT id, user_id, family_id::text, token_hash, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	revokeQuery = `
		UPDATE refresh_tokens SET revoked_at = $2
		WHERE id = $1 AND revoked_at IS NULL
	`
	revokeFamilyQuery = `
		UPDATE refresh_tokens SET revoked_at = $2
		WHERE family_id = $1 AND revoked_at IS NULL
	`
)
