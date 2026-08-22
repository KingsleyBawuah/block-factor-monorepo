-- +goose Up
-- +goose StatementBegin
CREATE TABLE cta_stops (
    id BIGSERIAL PRIMARY KEY,
    -- Maps to the source dataset's `map_id` (station-level), not `stop_id`
    -- (per-directional-platform) — see docs/DATA_SOURCES.md. One row per
    -- physical station; ingestion groups the source's per-direction rows
    -- and unions their per-line boolean flags into `lines`.
    source_id TEXT NOT NULL UNIQUE,
    stop_name TEXT NOT NULL,
    -- e.g. {Red, Blue} — dual-line stops power the S_transit dual-line bonus (PRD 5.2).
    lines TEXT[] NOT NULL,
    ada BOOLEAN NOT NULL DEFAULT false,
    geom GEOGRAPHY(POINT, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_cta_stops_geom ON cta_stops USING GIST (geom);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cta_stops;
-- +goose StatementEnd
