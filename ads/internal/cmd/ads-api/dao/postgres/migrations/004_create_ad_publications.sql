
CREATE TABLE ad_publications (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    UUID        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX idx_ad_publications_user_created_at
    ON ad_publications (user_id, created_at);
