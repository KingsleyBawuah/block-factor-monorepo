-- +goose Up
-- +goose StatementBegin
-- Generic key-value cache for non-spatial lookups (OTP2 route durations,
-- FCC broadband records) — ARCHITECTURE 5.4. Deliberately standard LOGGED
-- (not UNLOGGED): cache writes are near-zero volume, and LOGGED durability
-- protects against a costly thundering-herd refill after an unclean
-- restart, at negligible WAL overhead. See ARCHITECTURE 5.4 for the full
-- cost-math rationale.
CREATE TABLE cache_entries (
    cache_key TEXT PRIMARY KEY,
    cache_value JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_cache_entries_expires_at ON cache_entries (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cache_entries;
-- +goose StatementEnd
