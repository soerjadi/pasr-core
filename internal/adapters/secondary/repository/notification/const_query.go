package notification

const insertQuery = `
	INSERT INTO notifications (user_id, kind, title, body, data)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id
`

const claimUnsentQuery = `
	SELECT id, user_id, kind, title, body, data
	FROM notifications
	WHERE sent_at IS NULL AND attempts < $2
	ORDER BY id
	LIMIT $1
	FOR UPDATE SKIP LOCKED
`

const markSentQuery = `UPDATE notifications SET sent_at = now(), last_error = NULL WHERE id = $1`

const markFailedQuery = `UPDATE notifications SET attempts = attempts + 1, last_error = $2 WHERE id = $1`
