package idempotency

// claimQuery waits on a conflicting row that another transaction inserted but has not committed.
const claimQuery = `
	INSERT INTO idempotency_keys (user_id, key, request_hash)
	VALUES ($1, $2, $3)
	ON CONFLICT (user_id, key) DO NOTHING
`

// getQuery runs as a new statement, so under READ COMMITTED it sees the row that made claimQuery skip.
const getQuery = `
	SELECT COALESCE(order_id, 0), request_hash
	FROM idempotency_keys
	WHERE user_id = $1 AND key = $2
`

const attachQuery = `UPDATE idempotency_keys SET order_id = $3 WHERE user_id = $1 AND key = $2`
