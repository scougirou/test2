-- One row per ad published through POST /ads. It backs the publication quota and is deliberately independent of the ads table:
-- deleting an ad must not give quota back, and bulk imports are not recorded here.
CREATE TABLE ad_publications (
    id           BIGSERIAL   PRIMARY KEY,
    user_id      UUID        NOT NULL,
    published_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX ad_publications_user_id_published_at_idx ON ad_publications (user_id, published_at);

-- Serves the default ordering of GET /ads.
CREATE INDEX ads_created_at_id_idx ON ads (created_at DESC, id DESC);
