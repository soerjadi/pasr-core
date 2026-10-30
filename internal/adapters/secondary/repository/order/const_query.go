package order

// nextNumberQuery is one statement: the upsert takes the counter row lock, so concurrent callers
// queue on it and each gets a distinct value.
const nextNumberQuery = `
	INSERT INTO order_counters (day, last) VALUES ($1, 1)
	ON CONFLICT (day) DO UPDATE SET last = order_counters.last + 1
	RETURNING last
`

// decrementStockQuery is a plain conditional UPDATE: the row lock and the re-checked WHERE make it
// safe without a prior read. Reaching 0 marks the product OUT_OF_STOCK (as a seller edit would).
const decrementStockQuery = `
	UPDATE products
	SET stock = stock - $2,
	    status = CASE WHEN stock - $2 = 0 THEN 'OUT_OF_STOCK' ELSE status END
	WHERE id = $1 AND stock >= $2 AND status = 'AVAILABLE' AND deleted_at IS NULL
`

const insertOrderQuery = `
	INSERT INTO orders (number, buyer_id, total, status, created_at)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id
`

const insertSellerOrderQuery = `
	INSERT INTO seller_orders (order_id, store_id, status, subtotal, accept_by, store_name, pickup_address,
	                           opening_hours, whatsapp, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
	RETURNING id
`

// insertItemsQuery inserts all items of one seller-order in one round trip.
const insertItemsQuery = `
	INSERT INTO order_items (seller_order_id, product_id, name, image, quantity, unit_price, line_total)
	SELECT $1, t.product_id, t.name, t.image::jsonb, t.quantity, t.unit_price, t.line_total
	FROM unnest($2::bigint[], $3::text[], $4::text[], $5::int[], $6::bigint[], $7::bigint[])
	     AS t(product_id, name, image, quantity, unit_price, line_total)
`

// insertHistoryQuery writes one row per seller-order id in $2; a NULL element is the order-level entry.
const insertHistoryQuery = `
	INSERT INTO order_status_history (order_id, seller_order_id, status, at)
	SELECT $1, so, $3, $4 FROM unnest($2::bigint[]) AS so
`

const orderColumns = `id, number, status, created_at, total, payment_method`

const getQuery = `SELECT ` + orderColumns + ` FROM orders WHERE buyer_id = $1 AND id = $2`

// listQuery walks orders_buyer_idx (buyer_id, id DESC); the status filter is applied on the way.
const listQuery = `
	SELECT ` + orderColumns + `
	FROM orders
	WHERE buyer_id = $1
	  AND ($2::text[] IS NULL OR status = ANY($2))
	  AND ($3::bigint = 0 OR id < $3)
	ORDER BY id DESC
	LIMIT $4
`

const sellerOrdersQuery = `
	SELECT id, order_id, store_id, store_name, whatsapp, pickup_address, opening_hours, status, note, subtotal
	FROM seller_orders
	WHERE order_id = ANY($1)
	ORDER BY order_id, id
`

const itemsQuery = `
	SELECT oi.id, oi.seller_order_id, oi.product_id, oi.name, oi.image, oi.quantity, oi.unit_price, oi.line_total,
	       EXISTS (SELECT 1 FROM reviews r WHERE r.order_item_id = oi.id)
	FROM order_items oi
	JOIN seller_orders so ON so.id = oi.seller_order_id
	WHERE so.order_id = ANY($1)
	ORDER BY oi.id
`

// historyQuery returns the buyer-facing (order-level) history; seller-order rows have seller_order_id set.
const historyQuery = `
	SELECT order_id, status, at
	FROM order_status_history
	WHERE order_id = ANY($1) AND seller_order_id IS NULL
	ORDER BY order_id, at, id
`

// statementTimeoutQuery bounds every later statement, lock waits included, of the current transaction.
// SET does not take bind parameters, so set_config is used with is_local = true.
const statementTimeoutQuery = `SELECT set_config('statement_timeout', $1, true)`
