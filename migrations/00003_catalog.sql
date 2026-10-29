-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

INSERT INTO categories (name, icon_key, sort_order) VALUES
    ('Makanan & Minuman',      'food',        1),
    ('Fashion & Aksesoris',    'fashion',     2),
    ('Kerajinan & Dekorasi',   'craft',       3),
    ('Pertanian & Hasil Bumi', 'farm',        4),
    ('Kecantikan & Perawatan', 'beauty',      5),
    ('Elektronik & Aksesoris', 'electronics', 6),
    ('Rumah & Dapur',          'home',        7),
    ('Otomotif',               'auto',        8),
    ('Lainnya',                'other',       9);

CREATE TABLE uploads (
    id         BIGSERIAL PRIMARY KEY,
    owner_id   BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    keys       JSONB       NOT NULL,
    attached   BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX uploads_owner_idx ON uploads (owner_id);

CREATE TABLE stores (
    id              BIGSERIAL PRIMARY KEY,
    owner_id        BIGINT      NOT NULL UNIQUE REFERENCES users (id),
    name            TEXT        NOT NULL,
    category_id     BIGINT      NOT NULL REFERENCES categories (id),
    owner_name      TEXT        NOT NULL,
    kelurahan       TEXT        NOT NULL,
    address         TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    photo_upload_id BIGINT      REFERENCES uploads (id),
    status          TEXT        NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'VERIFIED', 'SUSPENDED')),
    is_open         BOOLEAN     NOT NULL DEFAULT true,
    whatsapp        TEXT        NOT NULL,
    hours           JSONB,
    rating_sum      BIGINT      NOT NULL DEFAULT 0,
    rating_count    INT         NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX stores_status_idx ON stores (status);

CREATE TABLE products (
    id             BIGSERIAL PRIMARY KEY,
    store_id       BIGINT      NOT NULL REFERENCES stores (id),
    category_id    BIGINT      NOT NULL REFERENCES categories (id),
    name           TEXT        NOT NULL,
    description    TEXT        NOT NULL DEFAULT '',
    price          BIGINT      NOT NULL CHECK (price > 0),
    stock          INT         NOT NULL CHECK (stock >= 0),
    status         TEXT        NOT NULL DEFAULT 'AVAILABLE' CHECK (status IN ('AVAILABLE', 'OUT_OF_STOCK', 'HIDDEN')),
    featured       BOOLEAN     NOT NULL DEFAULT false,
    halal_cert_no  TEXT,
    halal_verified BOOLEAN     NOT NULL DEFAULT false,
    rating_sum     BIGINT      NOT NULL DEFAULT 0,
    rating_count   INT         NOT NULL DEFAULT 0,
    deleted_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX products_name_trgm_idx ON products USING GIN (lower(name) gin_trgm_ops);
CREATE INDEX products_store_status_idx ON products (store_id, status);
CREATE INDEX products_category_idx ON products (category_id);

CREATE TABLE product_images (
    product_id BIGINT NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    upload_id  BIGINT NOT NULL REFERENCES uploads (id),
    position   INT    NOT NULL,
    PRIMARY KEY (product_id, position)
);

CREATE TABLE favorites (
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    product_id BIGINT      NOT NULL REFERENCES products (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, product_id)
);

CREATE TABLE follows (
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    store_id   BIGINT      NOT NULL REFERENCES stores (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, store_id)
);

-- +goose Down
DROP TABLE follows;
DROP TABLE favorites;
DROP TABLE product_images;
DROP TABLE products;
DROP TABLE stores;
DROP TABLE uploads;
-- Reset ids so a later Up seeds ids 1..9 again.
TRUNCATE categories RESTART IDENTITY;
DROP EXTENSION IF EXISTS pg_trgm;
