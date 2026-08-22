-- +goose Up
-- +goose StatementBegin
-- Powers S_flood (PRD 5.7). Zone designations are FEMA's official, fixed
-- national standard (not a Chicago-specific dataset quirk), confirmed
-- against FEMA's own published glossary — see docs/DATA_SOURCES.md.
CREATE TABLE fema_flood_zones (
    id BIGSERIAL PRIMARY KEY,
    source_id TEXT NOT NULL UNIQUE,
    zone_designation TEXT NOT NULL CHECK (
        zone_designation IN (
            'A', 'AE', 'AH', 'AO', 'AR', 'A99', 'V', 'VE', 'X', 'D'
        )
    ),
    geom GEOGRAPHY(MULTIPOLYGON, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_fema_flood_zones_geom ON fema_flood_zones USING GIST (geom);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS fema_flood_zones;
-- +goose StatementEnd
