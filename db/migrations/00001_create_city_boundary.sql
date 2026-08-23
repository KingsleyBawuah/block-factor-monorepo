-- +goose Up
-- +goose StatementBegin
CREATE TABLE city_boundary (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL DEFAULT 'Chicago',
    geom GEOGRAPHY(MULTIPOLYGON, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
-- Powers the OUT_OF_COVERAGE hard-reject: every scoring request is checked
-- with ST_Covers(geom, origin) before any factor math runs.
CREATE INDEX idx_city_boundary_geom ON city_boundary USING GIST (geom);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS city_boundary;
-- +goose StatementEnd
