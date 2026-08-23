-- +goose Up
-- +goose StatementBegin
-- Fully derived from GTFS stop_times AM-peak headway analysis (ARCHITECTURE
-- 11.2) — no natural source_id, since this is recomputed wholesale each
-- ingestion cycle rather than upserted row-by-row.
CREATE TABLE bus_corridors (
    id BIGSERIAL PRIMARY KEY,
    route_id TEXT NOT NULL,
    route_name TEXT NOT NULL,
    mean_headway_minutes NUMERIC(5, 2) NOT NULL,
    geom GEOGRAPHY(MULTILINESTRING, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_bus_corridors_geom ON bus_corridors USING GIST (geom);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS bus_corridors;
-- +goose StatementEnd
