-- +goose Up
CREATE TABLE orders (
    id             BIGSERIAL PRIMARY KEY,
    number         TEXT        NOT NULL UNIQUE,
    buyer_id       BIGINT      NOT NULL REFERENCES users (id),
    total          BIGINT      NOT NULL,
    payment_method TEXT        NOT NULL DEFAULT 'CASH_AT_PICKUP' CHECK (payment_method IN ('CASH_AT_PICKUP')),
    status         TEXT        NOT NULL CHECK (status IN ('CREATED', 'SELLER_PROCESSING', 'READY_FOR_PICKUP', 'COMPLETED', 'CANCELLED')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX orders_buyer_idx ON orders (buyer_id, id DESC);

CREATE TABLE seller_orders (
    id             BIGSERIAL PRIMARY KEY,
    order_id       BIGINT      NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    store_id       BIGINT      NOT NULL REFERENCES stores (id),
    status         TEXT        NOT NULL CHECK (status IN ('CREATED', 'SELLER_PROCESSING', 'READY_FOR_PICKUP', 'COMPLETED', 'CANCELLED')),
    subtotal       BIGINT      NOT NULL,
    note           TEXT        NOT NULL DEFAULT '',
    accept_by      TIMESTAMPTZ,
    store_name     TEXT        NOT NULL,
    pickup_address TEXT        NOT NULL,
    opening_hours  TEXT        NOT NULL DEFAULT '',
    whatsapp       TEXT        NOT NULL,
    cancel_reason  TEXT,
    cancel_note    TEXT,
    cancelled_by   TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX seller_orders_store_status_idx ON seller_orders (store_id, status, created_at);
CREATE INDEX seller_orders_order_idx ON seller_orders (order_id);
CREATE INDEX seller_orders_accept_by_idx ON seller_orders (accept_by) WHERE status = 'CREATED';

CREATE TABLE order_items (
    id              BIGSERIAL PRIMARY KEY,
    seller_order_id BIGINT NOT NULL REFERENCES seller_orders (id) ON DELETE CASCADE,
    product_id      BIGINT NOT NULL REFERENCES products (id),
    name            TEXT   NOT NULL,
    image           JSONB,
    quantity        INT    NOT NULL CHECK (quantity > 0),
    unit_price      BIGINT NOT NULL,
    line_total      BIGINT NOT NULL
);
CREATE INDEX order_items_seller_order_idx ON order_items (seller_order_id);
CREATE INDEX order_items_product_idx ON order_items (product_id);

CREATE TABLE order_status_history (
    id              BIGSERIAL PRIMARY KEY,
    order_id        BIGINT      NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    seller_order_id BIGINT      REFERENCES seller_orders (id) ON DELETE CASCADE,
    status          TEXT        NOT NULL,
    at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX order_status_history_order_idx ON order_status_history (order_id, at);

CREATE TABLE reviews (
    id            BIGSERIAL PRIMARY KEY,
    order_item_id BIGINT      NOT NULL UNIQUE REFERENCES order_items (id),
    product_id    BIGINT      NOT NULL REFERENCES products (id),
    store_id      BIGINT      NOT NULL REFERENCES stores (id),
    buyer_id      BIGINT      NOT NULL REFERENCES users (id),
    rating        INT         NOT NULL CHECK (rating BETWEEN 1 AND 5),
    text          TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX reviews_store_idx ON reviews (store_id, id DESC);
CREATE INDEX reviews_product_idx ON reviews (product_id, id DESC);

CREATE TABLE order_counters (
    day  DATE PRIMARY KEY,
    last INT  NOT NULL
);

CREATE TABLE idempotency_keys (
    user_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key          TEXT        NOT NULL,
    request_hash TEXT        NOT NULL,
    order_id     BIGINT      NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, key)
);

-- +goose Down
DROP TABLE idempotency_keys;
DROP TABLE order_counters;
DROP TABLE reviews;
DROP TABLE order_status_history;
DROP TABLE order_items;
DROP TABLE seller_orders;
DROP TABLE orders;
