-- +goose Up
-- +goose StatementBegin
-- Powers S_bike (PRD 5.11). route_type values verified against the live
-- Chicago Data Portal dataset (hvv9-38ut) via a full group-by count, not a
-- sample — see docs/DATA_SOURCES.md. Note there is no "trail" category
-- here: off-street trails (The 606, Lakefront Trail) live in `parks` as
-- park_class = 'LINEAR PARK', a completely separate dataset.
--
-- Directional network-bonus orientation (north-south vs. east-west) is
-- computed at query time via ST_Azimuth on the geometry, not stored here —
-- keeps it always in sync with the source geometry with no extra
-- ingestion-time computation.
CREATE TABLE bike_routes (
    id BIGSERIAL PRIMARY KEY,
    source_id TEXT NOT NULL UNIQUE,
    route_type TEXT NOT NULL CHECK (
        route_type IN (
            'Protected Bike Lane',
            'Neighborhood Greenway',
            'Buffered Bike Lane',
            'Bike Lane',
            'Marked Shared Lane'
        )
    ),
    street_name TEXT,
    geom GEOGRAPHY(MULTILINESTRING, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_bike_routes_geom ON bike_routes USING GIST (geom);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS bike_routes;
-- +goose StatementEnd
