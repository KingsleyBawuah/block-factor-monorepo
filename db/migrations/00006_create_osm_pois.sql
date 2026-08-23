-- +goose Up
-- +goose StatementBegin
-- Powers S_grocery (PRD 5.3). poi_type derived at ingestion from standard,
-- globally-stable OSM tags (shop=supermarket, shop=convenience,
-- amenity=pharmacy) — see docs/DATA_SOURCES.md for why this dataset was
-- treated as lower-risk than the Chicago-specific ones and not live-sampled
-- during the Phase 2 investigation.
CREATE TABLE osm_pois (
    id BIGSERIAL PRIMARY KEY,
    source_id TEXT NOT NULL UNIQUE, -- OSM node/way id
    poi_name TEXT,
    poi_type TEXT NOT NULL CHECK (
        poi_type IN ('supermarket', 'convenience', 'pharmacy')
    ),
    geom GEOGRAPHY(POINT, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_osm_pois_geom ON osm_pois USING GIST (geom);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS osm_pois;
-- +goose StatementEnd
