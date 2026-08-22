-- +goose Up
-- +goose StatementBegin
-- Powers the S_safety percentile normalization (PRD 5.9). Fully derived
-- from cpd_crimes each ingestion cycle via a staging-table + transactional
-- rename swap (ARCHITECTURE 11.3) — no source_id, no upserts, the whole
-- table is replaced atomically so live requests never read a partially
-- computed distribution.
CREATE TABLE safety_grid_percentiles (
    id BIGSERIAL PRIMARY KEY,
    cell_geom GEOGRAPHY(POLYGON, 4326) NOT NULL,
    penalty_score NUMERIC NOT NULL,
    percentile NUMERIC(5, 2) NOT NULL CHECK (percentile BETWEEN 0 AND 100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_safety_grid_percentiles_geom ON safety_grid_percentiles USING GIST (cell_geom);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS safety_grid_percentiles;
-- +goose StatementEnd
