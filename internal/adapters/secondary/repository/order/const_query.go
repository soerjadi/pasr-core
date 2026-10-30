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

// lockSellerOrderQuery locks only the seller-order row; $2 = 0 skips the store check (operator, system).
const lockSellerOrderQuery = `
	SELECT so.id, so.order_id, o.number, o.buyer_id, so.store_id, so.store_name, s.owner_id, so.status,
	       so.accept_by, COALESCE(so.cancel_reason, '')
	FROM seller_orders so
	JOIN orders o ON o.id = so.order_id
	JOIN stores s ON s.id = so.store_id
	WHERE so.id = $1 AND ($2::bigint = 0 OR so.store_id = $2)
	FOR UPDATE OF so
`

// updateSellerOrderQuery applies one transition and appends its seller-order history row.
// NULL parameters keep the stored note and cancellation fields.
const updateSellerOrderQuery = `
	WITH u AS (
		UPDATE seller_orders
		SET status = $2, updated_at = $3,
		    note = COALESCE($4, note),
		    cancel_reason = COALESCE($5, cancel_reason),
		    cancel_note = COALESCE($6, cancel_note),
		    cancelled_by = COALESCE($7, cancelled_by)
		WHERE id = $1
		RETURNING order_id
	)
	INSERT INTO order_status_history (order_id, seller_order_id, status, at)
	SELECT order_id, $1, $2, $3 FROM u
`

// lockRestoreProductsQuery locks the seller-order's products in ascending id order, the same
// order createOrder uses, so the two never deadlock.
const lockRestoreProductsQuery = `
	SELECT id FROM products
	WHERE id IN (SELECT product_id FROM order_items WHERE seller_order_id = $1)
	ORDER BY id
	FOR UPDATE
`

// restoreStockQuery is a plain UPDATE: it gives the items' quantities back and returns a product
// that was OUT_OF_STOCK to AVAILABLE once it has stock again.
const restoreStockQuery = `
	UPDATE products p
	SET stock = p.stock + q.qty,
	    status = CASE WHEN p.status = 'OUT_OF_STOCK' AND p.stock + q.qty > 0 THEN 'AVAILABLE' ELSE p.status END
	FROM (SELECT product_id, sum(quantity) AS qty FROM order_items WHERE seller_order_id = $1 GROUP BY product_id) q
	WHERE p.id = q.product_id
`

// lockOrderQuery takes FOR NO KEY UPDATE: it serialises status recomputation of one order but does
// not block the KEY SHARE locks that history inserts take through their foreign key.
const lockOrderQuery = `SELECT status FROM orders WHERE id = $1 FOR NO KEY UPDATE`

const sellerStatusesQuery = `SELECT status FROM seller_orders WHERE order_id = $1 ORDER BY id`

// setOrderStatusQuery stores the derived order status and appends the order-level history row.
const setOrderStatusQuery = `
	WITH u AS (UPDATE orders SET status = $2 WHERE id = $1 RETURNING id)
	INSERT INTO order_status_history (order_id, seller_order_id, status, at)
	SELECT id, NULL, $2, $3 FROM u
`

const storeOrderColumns = `
	SELECT so.id, o.number, so.status, so.created_at, so.accept_by, u.name, so.note, so.subtotal
	FROM seller_orders so
	JOIN orders o ON o.id = so.order_id
	JOIN users u ON u.id = o.buyer_id
`

// getStoreOrderQuery: $2 = 0 reads a seller-order of any store.
const getStoreOrderQuery = storeOrderColumns + `WHERE so.id = $1 AND ($2::bigint = 0 OR so.store_id = $2)`

// listStoreOrdersQuery walks seller_orders_store_status_id_idx or seller_orders_store_id_idx.
const listStoreOrdersQuery = storeOrderColumns + `
	WHERE so.store_id = $1 AND so.status = ANY($2) AND ($3::bigint = 0 OR so.id < $3)
	ORDER BY so.id DESC
	LIMIT $4
`

// listAllStoreOrdersQuery pages the seller-orders of every store newest first; with a status it
// walks seller_orders_status_id_idx, without one the primary key.
const listAllStoreOrdersQuery = `
	SELECT so.id, o.number, so.status, so.created_at, so.accept_by, u.name, so.note, so.subtotal, so.store_id, so.store_name
	FROM seller_orders so
	JOIN orders o ON o.id = so.order_id
	JOIN users u ON u.id = o.buyer_id
	WHERE ($1 = '' OR so.status = $1) AND ($2::bigint = 0 OR so.id < $2)
	ORDER BY so.id DESC
	LIMIT $3
`

const storeOrderItemsQuery = `
	SELECT oi.id, oi.seller_order_id, oi.product_id, oi.name, oi.image, oi.quantity, oi.unit_price, oi.line_total,
	       EXISTS (SELECT 1 FROM reviews r WHERE r.order_item_id = oi.id)
	FROM order_items oi
	WHERE oi.seller_order_id = ANY($1)
	ORDER BY oi.id
`

const storeOrderHistoryQuery = `
	SELECT seller_order_id, status, at
	FROM order_status_history
	WHERE seller_order_id = ANY($1)
	ORDER BY seller_order_id, at, id
`

// countStoreOrdersQuery counts every seller tab in one pass over the store's seller-orders.
const countStoreOrdersQuery = `
	SELECT count(*) FILTER (WHERE status = 'CREATED'),
	       count(*) FILTER (WHERE status IN ('SELLER_PROCESSING', 'READY_FOR_PICKUP')),
	       count(*) FILTER (WHERE status IN ('COMPLETED', 'CANCELLED'))
	FROM seller_orders
	WHERE store_id = $1
`

// overdueQuery uses seller_orders_accept_by_idx; SKIP LOCKED passes over rows being acted on.
const overdueQuery = `
	SELECT id FROM seller_orders
	WHERE status = 'CREATED' AND accept_by <= $1
	ORDER BY accept_by, id
	LIMIT $2
	FOR UPDATE SKIP LOCKED
`

// reviewTargetQuery matches only items of the buyer's own orders.
const reviewTargetQuery = `
	SELECT oi.id, oi.product_id, so.store_id, so.status
	FROM order_items oi
	JOIN seller_orders so ON so.id = oi.seller_order_id
	JOIN orders o ON o.id = so.order_id
	WHERE oi.id = $2 AND o.buyer_id = $1
`
