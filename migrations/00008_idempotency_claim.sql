-- +goose Up
-- createOrder claims its Idempotency-Key first (the unique key serializes concurrent requests)
-- and attaches the order id later in the same transaction, so order_id is empty until then.
-- A committed row always has an order: the claim and the attach commit or roll back together.
ALTER TABLE idempotency_keys ALTER COLUMN order_id DROP NOT NULL;

-- +goose Down
DELETE FROM idempotency_keys WHERE order_id IS NULL;
ALTER TABLE idempotency_keys ALTER COLUMN order_id SET NOT NULL;
