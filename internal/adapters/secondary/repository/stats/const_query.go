package stats

// countCreatedQuery walks seller_orders_store_status_id_idx / seller_orders_store_id_idx per store.
const countCreatedQuery = `
	SELECT count(*)::int
	FROM seller_orders
	WHERE store_id = $1 AND status <> 'CANCELLED' AND created_at >= $2 AND created_at < $3
`

// completedFrom joins a COMPLETED seller-order to its COMPLETED history row through
// order_status_history_order_idx (order_id, at).
const completedFrom = `
	FROM seller_orders so
	JOIN order_status_history h
	  ON h.order_id = so.order_id AND h.seller_order_id = so.id AND h.status = 'COMPLETED'
	WHERE so.store_id = $1 AND so.status = 'COMPLETED' AND h.at >= $2 AND h.at < $3
`

const completedByDayQuery = `
	SELECT to_char(h.at AT TIME ZONE $4, 'YYYY-MM-DD'), sum(so.subtotal)::bigint, count(*)::int
` + completedFrom + `
	GROUP BY 1
	ORDER BY 1
`

const topProductsQuery = `
	SELECT oi.product_id, (array_agg(oi.name ORDER BY h.at DESC, oi.id DESC))[1], sum(oi.quantity)::bigint
	FROM seller_orders so
	JOIN order_status_history h
	  ON h.order_id = so.order_id AND h.seller_order_id = so.id AND h.status = 'COMPLETED'
	JOIN order_items oi ON oi.seller_order_id = so.id
	WHERE so.store_id = $1 AND so.status = 'COMPLETED' AND h.at >= $2 AND h.at < $3
	GROUP BY oi.product_id
	ORDER BY 3 DESC, oi.product_id
	LIMIT $4
`
