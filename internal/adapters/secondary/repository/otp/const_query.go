package otp

const (
	createQuery = `
		INSERT INTO otp_codes (phone, ip, code_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	invalidateActiveQuery = `
		UPDATE otp_codes SET consumed_at = $2
		WHERE phone = $1 AND consumed_at IS NULL
	`
	countByPhoneSinceQuery = `
		SELECT count(*) FROM otp_codes WHERE phone = $1 AND created_at >= $2
	`
	countByIPSinceQuery = `
		SELECT count(*) FROM otp_codes WHERE ip = $1 AND created_at >= $2
	`
	getActiveQuery = `
		SELECT id, phone, code_hash, expires_at, attempts
		FROM otp_codes
		WHERE phone = $1 AND consumed_at IS NULL
		ORDER BY id DESC
		LIMIT 1
	`
	incrementAttemptsQuery = `
		UPDATE otp_codes SET attempts = attempts + 1
		WHERE id = $1
		RETURNING attempts
	`
	consumeQuery = `
		UPDATE otp_codes SET consumed_at = $2
		WHERE id = $1 AND consumed_at IS NULL
	`
)
