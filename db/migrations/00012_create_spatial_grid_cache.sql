-- +goose Up
-- +goose StatementBegin
-- Google Places 400m grid-cell cache (ARCHITECTURE 5.4/5.5), 30-day TTL per
-- Google Maps Platform Terms (THIRD_PARTY_REQUIREMENTS 2). Cell identity is
-- the integer (grid_x, grid_y) pair, not geometry equality — floating-point
-- polygon comparison would be fragile as a uniqueness key. `cell_geom` is
-- kept purely for the spatial join (which cell does this coordinate fall
-- into), computed from grid_x/grid_y at ingestion time.
CREATE TABLE spatial_grid_cache (
    id BIGSERIAL PRIMARY KEY,
    grid_x INTEGER NOT NULL,
    grid_y INTEGER NOT NULL,
    cell_geom GEOGRAPHY(POLYGON, 4326) NOT NULL,
    places_response JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (grid_x, grid_y)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_spatial_grid_cache_geom ON spatial_grid_cache USING GIST (cell_geom);
-- +goose StatementEnd

-- +goose StatementBegin
-- Supports TTL cleanup queries (WHERE expires_at < now()) without a full
-- table scan.
CREATE INDEX idx_spatial_grid_cache_expires_at ON spatial_grid_cache (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS spatial_grid_cache;
-- +goose StatementEnd
