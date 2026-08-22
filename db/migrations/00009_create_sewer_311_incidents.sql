-- +goose Up
-- +goose StatementBegin
-- Powers the S_flood 311 sewer-backup penalty (PRD 5.7). Sourced from the
-- unified 311 dataset (v6vf-nfxy) filtered to sr_type = 'Water in Basement
-- Complaint' — confirmed as the real, exact string via a live query; see
-- docs/DATA_SOURCES.md. That filter is ingestion-time logic, not enforced
-- here, since this table only stores incidents already known to qualify.
CREATE TABLE sewer_311_incidents (
    id BIGSERIAL PRIMARY KEY,
    source_id TEXT NOT NULL UNIQUE,
    incident_date TIMESTAMPTZ NOT NULL,
    geom GEOGRAPHY(POINT, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_sewer_311_incidents_geom ON sewer_311_incidents USING GIST (geom);
-- +goose StatementEnd

-- +goose StatementBegin
-- Supports the "trailing 24 months" window filter (PRD 5.7) without a full
-- table scan.
CREATE INDEX idx_sewer_311_incidents_date ON sewer_311_incidents (incident_date);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sewer_311_incidents;
-- +goose StatementEnd
