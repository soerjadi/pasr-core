-- +goose Up
CREATE TABLE users (
    id         BIGSERIAL PRIMARY KEY,
    phone      TEXT        NOT NULL UNIQUE,
    name       TEXT        NOT NULL DEFAULT '',
    role       TEXT        NOT NULL DEFAULT 'buyer' CHECK (role IN ('buyer', 'operator')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE otp_codes (
    id          BIGSERIAL PRIMARY KEY,
    phone       TEXT        NOT NULL,
    ip          TEXT        NOT NULL DEFAULT '',
    code_hash   TEXT        NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    attempts    INT         NOT NULL DEFAULT 0,
    consumed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX otp_codes_phone_created_idx ON otp_codes (phone, created_at);
CREATE INDEX otp_codes_ip_created_idx ON otp_codes (ip, created_at);

CREATE TABLE refresh_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    family_id  UUID        NOT NULL,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);

CREATE TABLE devices (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    fcm_token  TEXT        NOT NULL UNIQUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX devices_user_idx ON devices (user_id);

-- +goose Down
DROP TABLE devices;
DROP TABLE refresh_tokens;
DROP TABLE otp_codes;
DROP TABLE users;
