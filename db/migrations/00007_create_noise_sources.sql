-- +goose Up
-- +goose StatementBegin
-- Powers S_noise (PRD 5.6). `grade` only applies to CTA track segments —
-- ARCHITECTURE 5.6: track penalties apply only to elevated and at-grade
-- segments; subway (underground) segments are exempt. Expressway/arterial
-- rows leave `grade` null since it doesn't apply to them.
CREATE TABLE noise_sources (
    id BIGSERIAL PRIMARY KEY,
    source_id TEXT NOT NULL UNIQUE,
    source_type TEXT NOT NULL CHECK (
        source_type IN ('cta_track', 'expressway', 'arterial')
    ),
    grade TEXT CHECK (grade IN ('elevated', 'at_grade', 'underground')),
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
