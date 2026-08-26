-- +goose Up
-- +goose StatementBegin
-- Powers the Phase 4 CTA Rail map layer (docs/IMPLEMENTATION_PHASES.md
-- Phase 4) -- the only real per-line rail track geometry available.
-- cta_stops has only station points (with a `lines` array); noise_sources'
-- CTA track geometry is explicitly line-agnostic. See
-- docs/DATA_SOURCES.md "CTA 'L' Lines" and ARCHITECTURE.md Decision Log
-- row 31.
--
-- `lines` is parsed from the source's messy comma-separated `lines` field
-- (e.g. "Brown, Purple (Express), Red") into cta_stops.lines' vocabulary
-- (Red, Blue, Brown, Green, Orange, Pink, Purple, Yellow).
-- `express` is a single per-segment flag, true if any line on this segment
-- runs express service here (the source doesn't attribute which specific
-- line -- see docs/DATA_SOURCES.md). `grade` mirrors the source's `type`
-- field (Subway vs. Elevated-or-At-Grade only) -- coarser than
-- noise_sources' three-way underground/elevated/at_grade split, so the two
-- grade vocabularies are intentionally not unified.
CREATE TABLE cta_rail_lines (
    id BIGSERIAL PRIMARY KEY,
    source_id TEXT NOT NULL UNIQUE,
    lines TEXT[] NOT NULL,
    express BOOLEAN NOT NULL DEFAULT false,
    grade TEXT NOT NULL CHECK (grade IN ('subway', 'elevated_or_at_grade')),
    geom GEOGRAPHY(MULTILINESTRING, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_cta_rail_lines_geom ON cta_rail_lines USING GIST (geom);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cta_rail_lines;
-- +goose StatementEnd
