-- +goose Up
-- +goose StatementBegin
-- Powers S_safety (PRD 5.9). The source dataset (ijzp-q8t2) does not
-- provide a violent/property flag — `primary_type` is free text (e.g.
-- "BATTERY", "CRIMINAL DAMAGE") and the real classification codes are
-- `iucr`/`fbi_code` — see docs/DATA_SOURCES.md. Both the raw classification
-- and our derived category are stored: the raw fields let the
-- violent/property mapping be audited or refined later without needing to
-- re-ingest from source.
--
-- source_id maps to the source's `id` field (Socrata's stable row
-- identifier), not `case_number` (a human-facing but less stable key).
--
-- category is intentionally a closed 2-value set: primary_type has 30+ real
-- values (narcotics, gambling, liquor law violations, etc.) that are neither
-- violent nor property, and S_safety (PRD 5.9) only scores those two. The
-- ingestion CLI filters out non-matching rows before insert — see
-- docs/DATA_SOURCES.md "Category coverage".
CREATE TABLE cpd_crimes (
    id BIGSERIAL PRIMARY KEY,
    source_id TEXT NOT NULL UNIQUE,
    crime_date TIMESTAMPTZ NOT NULL,
    primary_type TEXT NOT NULL,
    iucr TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('violent', 'property')),
    geom GEOGRAPHY(POINT, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_cpd_crimes_geom ON cpd_crimes USING GIST (geom);
-- +goose StatementEnd

-- +goose StatementBegin
-- Supports the "trailing 12 months" window filter (PRD 5.9) without a full
-- table scan.
CREATE INDEX idx_cpd_crimes_date ON cpd_crimes (crime_date);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cpd_crimes;
-- +goose StatementEnd
