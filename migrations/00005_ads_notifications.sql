-- +goose Up
CREATE TABLE ad_campaigns (
    id         BIGSERIAL PRIMARY KEY,
    placement  TEXT        NOT NULL CHECK (placement IN ('home_partner_carousel', 'home_sponsored_card', 'post_purchase', 'wait_state')),
    advertiser TEXT        NOT NULL,
    title      TEXT        NOT NULL,
    subtitle   TEXT        NOT NULL DEFAULT '',
    image      JSONB       NOT NULL,
    target_url TEXT        NOT NULL,
    label      TEXT        NOT NULL,
    priority   INT         NOT NULL DEFAULT 0,
    active     BOOLEAN     NOT NULL DEFAULT true,
    starts_at  TIMESTAMPTZ,
    ends_at    TIMESTAMPTZ
);
CREATE INDEX ad_campaigns_placement_idx ON ad_campaigns (placement, priority DESC) WHERE active;

INSERT INTO ad_campaigns (placement, advertiser, title, subtitle, image, target_url, label, priority) VALUES
    ('home_partner_carousel', 'Bank BRI', 'KUR Bank BRI', 'Modal usaha untuk UMKM dengan bunga ringan',
     '{"small":"https://placehold.co/320x320/DCE9F7/1B5E20/png?text=KUR+BRI","medium":"https://placehold.co/640x640/DCE9F7/1B5E20/png?text=KUR+BRI","large":"https://placehold.co/1080x1080/DCE9F7/1B5E20/png?text=KUR+BRI"}',
     'https://bri.co.id', 'Kemitraan', 20),
    ('home_partner_carousel', 'Pemerintah Kota', 'Mitra Unggulan UMKM', 'Program pendampingan usaha lokal',
     '{"small":"https://placehold.co/320x320/E8F5E9/1B5E20/png?text=Mitra","medium":"https://placehold.co/640x640/E8F5E9/1B5E20/png?text=Mitra","large":"https://placehold.co/1080x1080/E8F5E9/1B5E20/png?text=Mitra"}',
     'https://example.id/mitra', 'Kemitraan', 10),
    ('home_sponsored_card', 'Bank BRI', 'KUR Bank BRI', 'Ajukan pinjaman usaha dengan cepat',
     '{"small":"https://placehold.co/320x180/DCE9F7/1B5E20/png?text=KUR+BRI","medium":"https://placehold.co/640x360/DCE9F7/1B5E20/png?text=KUR+BRI","large":"https://placehold.co/1080x607/DCE9F7/1B5E20/png?text=KUR+BRI"}',
     'https://bri.co.id', 'Kemitraan', 0),
    ('post_purchase', 'J&T Express', 'Kirim paket dengan J&T', 'Pengiriman cepat ke seluruh Indonesia',
     '{"small":"https://placehold.co/320x180/FFF3E0/1B5E20/png?text=J%26T","medium":"https://placehold.co/640x360/FFF3E0/1B5E20/png?text=J%26T","large":"https://placehold.co/1080x607/FFF3E0/1B5E20/png?text=J%26T"}',
     'https://www.jet.co.id', 'Iklan', 0),
    ('wait_state', 'Indosat', 'Paket data Indosat', 'Tetap terhubung selama menunggu',
     '{"small":"https://placehold.co/320x180/FCE4EC/1B5E20/png?text=Indosat","medium":"https://placehold.co/640x360/FCE4EC/1B5E20/png?text=Indosat","large":"https://placehold.co/1080x607/FCE4EC/1B5E20/png?text=Indosat"}',
     'https://indosatooredoo.com', 'Iklan', 0);

CREATE TABLE notifications (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind       TEXT        NOT NULL,
    title      TEXT        NOT NULL,
    body       TEXT        NOT NULL,
    data       JSONB       NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at    TIMESTAMPTZ,
    attempts   INT         NOT NULL DEFAULT 0,
    last_error TEXT
);
CREATE INDEX notifications_unsent_idx ON notifications (id) WHERE sent_at IS NULL;

-- +goose Down
DROP TABLE notifications;
DROP TABLE ad_campaigns;
