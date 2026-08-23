-- +goose Up
-- +goose StatementBegin
-- Powers S_noise (PRD 5.6). `grade` only applies to CTA track segments —
-- ARCHITECTURE 5.6: track penalties apply only to elevated and at-grade
-- segments; subway (underground) segments are exempt. Expressway/arterial
-- rows leave `grade` null since it doesn't apply to them.
--
-- lanes/maxspeed_mph/has_turn_lane are captured from OSM at ingestion time
-- for every source_type, even though no scoring logic uses them yet, to
-- avoid a costly re-ingest later -- see PRD 5.6 "Future Factor — S_stroad
-- / Street Safety". All three are nullable rather than required: OSM's tag
-- coverage for these is genuinely incomplete on real Chicago roads (many
-- primary/secondary ways lack a lanes tag entirely), so treating them as
-- best-effort metadata avoids either fabricating values or being unable to
-- insert real, legitimate road segments.
CREATE TABLE noise_sources (
    id BIGSERIAL PRIMARY KEY,
    source_id TEXT NOT NULL UNIQUE,
    source_type TEXT NOT NULL CHECK (
        source_type IN ('cta_track', 'expressway', 'arterial')
    ),
    grade TEXT CHECK (grade IN ('elevated', 'at_grade', 'underground')),
    lanes SMALLINT,
    maxspeed_mph SMALLINT,
    has_turn_lane BOOLEAN,
    geom GEOGRAPHY(MULTILINESTRING, 4326) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT noise_sources_grade_only_for_cta CHECK (
        (source_type = 'cta_track' AND grade IS NOT NULL)
        OR (source_type <> 'cta_track' AND grade IS NULL)
    )
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_noise_sources_geom ON noise_sources USING GIST (geom);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS noise_sources;
-- +goose StatementEnd
