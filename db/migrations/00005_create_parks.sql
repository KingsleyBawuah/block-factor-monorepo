-- +goose Up
-- +goose StatementBegin
-- Real park_class values verified against the live Chicago Data Portal
-- dataset (ejsh-fztr, "CPD_Parks" — the actual populated one; two other
-- candidate resource IDs returned empty geometry, see docs/DATA_SOURCES.md):
-- NEIGHBORHOOD PARK, COMMUNITY PARK, REGIONAL PARK, MINI-PARK, PASSIVE PARK,
-- NATURE PRESERVE PARK, UNIMPROVED PARK, PARKWAY PARK, PARKWAYS PARK (both
-- spellings exist in the source — a real data-quality quirk, not a typo on
-- our end), MAGNET PARK, CITYWIDE PARK, LINEAR PARK, and PENDING (unresolved
-- classification). Deliberately NOT a CHECK constraint: 13 messy,
-- overlapping source values including a known duplicate-spelling quirk is a
-- poor fit for a rigid enum, and the only value scoring logic actually
-- depends on is 'LINEAR PARK' (the real source of the PRD 5.5 off-street
-- trail bonus — NOT bike_routes, which has no trail category at all).
CREATE TABLE parks (
    id BIGSERIAL PRIMARY KEY,
    source_id TEXT NOT NULL UNIQUE,
    park_name TEXT NOT NULL,
    park_class TEXT NOT NULL,
    acres NUMERIC(10, 2),
    geom GEOGRAPHY(MULTIPOLYGON, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_parks_geom ON parks USING GIST (geom);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS parks;
-- +goose StatementEnd
