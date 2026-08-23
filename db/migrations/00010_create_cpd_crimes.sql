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
-- category is intentionally a closed 4-value set: primary_type (and, for the
-- "OTHER OFFENSE"/"OFFENSE INVOLVING CHILDREN" catch-alls, the finer-grained
-- iucr code) has 30+ real top-level values plus dozens of iucr sub-codes,
-- many of which are neither violent, sexual, property, nor a quality-of-life
-- nuisance relevant to S_safety (PRD 5.9) — e.g. gambling, liquor licensing,
-- parole/probation compliance. The ingestion CLI filters those out before
-- insert rather than storing a 5th "excluded" value — see the full mapping
-- table in docs/DATA_SOURCES.md "Category coverage".
CREATE TABLE cpd_crimes (
    id BIGSERIAL PRIMARY KEY,
    source_id TEXT NOT NULL UNIQUE,
    crime_date TIMESTAMPTZ NOT NULL,
    primary_type TEXT NOT NULL,
    iucr TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('violent', 'sexual', 'property', 'nuisance')),
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
