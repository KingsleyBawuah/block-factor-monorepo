-- +goose Up
-- +goose StatementBegin
-- Powers the S_safety percentile normalization (PRD 5.9). Fully derived
-- from cpd_crimes each ingestion cycle via a staging-table + transactional
-- rename swap (ARCHITECTURE 11.3) — no source_id, no upserts, the whole
-- table is replaced atomically so live requests never read a partially
-- computed distribution. No spatial uniqueness constraint on cell_geom for
-- the same reason: uniqueness of the ~500m grid is guaranteed by the
-- generation algorithm, not enforced by the DB, since the whole table is
-- discarded and regenerated each cycle rather than upserted.
--
-- Raw per-category counts are stored alongside the combined penalty_score
-- (same "keep the raw inputs, not just the derived value" principle as
-- cpd_crimes.primary_type/iucr) — the category weights (PRD 5.9) have
-- already been revised once during Phase 2 design, so re-deriving
-- penalty_score from these counts if they change again shouldn't require
-- re-ingesting from cpd_crimes.
CREATE TABLE safety_grid_percentiles (
    id BIGSERIAL PRIMARY KEY,
    cell_geom GEOGRAPHY(POLYGON, 4326) NOT NULL,
    violent_count INTEGER NOT NULL DEFAULT 0,
    sexual_count INTEGER NOT NULL DEFAULT 0,
    property_count INTEGER NOT NULL DEFAULT 0,
    nuisance_count INTEGER NOT NULL DEFAULT 0,
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
